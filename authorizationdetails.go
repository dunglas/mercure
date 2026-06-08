package mercure

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Element-count caps on the authorization_details claim, mirroring the
// publish/subscribe query limits. A token exceeding any of them is rejected
// with a 400 status code.
const (
	maxMercureDetails = 1000 // type=="mercure" authorization details
	maxDetailTopics   = 1000 // entries per `topics` array
)

// authorizationDetailTypeMercure is the RFC 9396 authorization detail type
// defined by this protocol.
const authorizationDetailTypeMercure = "mercure"

// mercureAction is a value of the `actions` array of a mercure authorization
// detail.
type mercureAction string

const (
	actionPublish   mercureAction = "publish"
	actionSubscribe mercureAction = "subscribe"
)

// errInvalidAuthorizationDetail is returned when the authorization_details
// claim is malformed. The HTTP handlers map it to a 400 "invalid_request"
// response (no partial acceptance: one bad mercure detail rejects the token).
var errInvalidAuthorizationDetail = errors.New("invalid authorization_details claim")

// authorizationDetail is one entry of the RFC 9396 authorization_details
// claim. Entries whose Type is not "mercure" are ignored.
type authorizationDetail struct {
	Type    string          `json:"type"`
	Actions []mercureAction `json:"actions"`
	Topics  []detailTopic   `json:"topics"`
	Payload any             `json:"payload,omitempty"`
}

// detailTopic is one entry of a mercure authorization detail `topics` array.
// Only the object form {match, matchType?} is accepted; matchType is
// case-sensitive and defaults to Exact.
type detailTopic struct {
	topicMatcher
}

// MarshalJSON emits the object form {match, matchType}. Issuers normally mint
// tokens, but the hub round-trips them in tests.
func (d detailTopic) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(struct {
		Match     string      `json:"match"`
		MatchType MatcherType `json:"matchType,omitempty"`
	}{d.Pattern, d.Type})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal topic matcher: %w", err)
	}

	return b, nil
}

// UnmarshalJSON enforces the object form. A bare string (the deprecated claim
// shape) is rejected so that legacy tokens do not silently parse as Exact
// matchers.
func (d *detailTopic) UnmarshalJSON(data []byte) error {
	var obj struct {
		Match     string      `json:"match"`
		MatchType MatcherType `json:"matchType"`
	}

	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("%w: topic entries must be objects: %w", errInvalidAuthorizationDetail, err)
	}

	d.Pattern = obj.Match
	d.Type = obj.MatchType

	if d.Type == "" {
		d.Type = MatcherTypeExact
	}

	return nil
}

// validatedDetail is a parsed and validated mercure authorization detail.
type validatedDetail struct {
	publish   bool
	subscribe bool
	topics    []topicMatcher
	payload   any
}

// mercureAuthz holds the validated mercure authorization details of a token.
type mercureAuthz struct {
	details []validatedDetail
}

// validateAuthorizationDetails parses and validates the mercure entries of an
// authorization_details claim. Non-mercure entries are skipped. Any malformed
// mercure entry returns errInvalidAuthorizationDetail; the caller rejects the
// whole token (no partial acceptance).
func validateAuthorizationDetails(tss *TopicSelectorStore, raw []authorizationDetail) (*mercureAuthz, error) {
	authz := &mercureAuthz{}

	var count int

	for i := range raw {
		if raw[i].Type != authorizationDetailTypeMercure {
			continue
		}

		count++
		if count > maxMercureDetails {
			return nil, fmt.Errorf("%w: too many mercure authorization details (max %d)", errInvalidAuthorizationDetail, maxMercureDetails)
		}

		vd, err := validateMercureDetail(tss, raw[i])
		if err != nil {
			return nil, err
		}

		authz.details = append(authz.details, vd)
	}

	return authz, nil
}

func validateMercureDetail(tss *TopicSelectorStore, d authorizationDetail) (validatedDetail, error) {
	vd := validatedDetail{payload: d.Payload}

	if len(d.Actions) == 0 {
		return vd, fmt.Errorf("%w: a mercure detail must declare at least one action", errInvalidAuthorizationDetail)
	}

	for _, a := range d.Actions {
		switch a {
		case actionPublish:
			vd.publish = true
		case actionSubscribe:
			vd.subscribe = true
		default:
			return vd, fmt.Errorf("%w: unsupported action %q", errInvalidAuthorizationDetail, a)
		}
	}

	if len(d.Topics) == 0 {
		return vd, fmt.Errorf("%w: a mercure detail must declare at least one topic", errInvalidAuthorizationDetail)
	}

	if len(d.Topics) > maxDetailTopics {
		return vd, fmt.Errorf("%w: too many topics in a mercure detail (max %d)", errInvalidAuthorizationDetail, maxDetailTopics)
	}

	vd.topics = make([]topicMatcher, len(d.Topics))
	for i := range d.Topics {
		m := d.Topics[i].topicMatcher

		if len(m.Pattern) > maxPatternLength {
			return vd, fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, errPatternTooLong)
		}

		if !validProtocolString(m.Pattern) {
			return vd, fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, errInvalidMatcherValue)
		}

		// Only the protocol matcher types are valid in authorization details;
		// the internal deprecated type must not be reachable from a token.
		switch m.Type {
		case MatcherTypeExact, MatcherTypeURLPattern:
			if err := tss.validatePattern(m); err != nil {
				return vd, fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, err)
			}
		case deprecatedMatcherTypeName:
			// The internal deprecated type must never be reachable from a token.
			return vd, fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, ErrUnsupportedMatcherType)
		default:
			return vd, fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, ErrUnsupportedMatcherType)
		}

		vd.topics[i] = m
	}

	return vd, nil
}

// grants reports whether the token authorizes the given action on the topic.
func (a *mercureAuthz) grants(tss *TopicSelectorStore, action mercureAction, topic string) bool {
	if a == nil {
		return false
	}

	single := []string{topic}

	for i := range a.details {
		if !a.details[i].hasAction(action) {
			continue
		}

		for _, m := range a.details[i].topics {
			if tss.matchMatcher(single, m) {
				return true
			}
		}
	}

	return false
}

// grantsAll reports whether the token authorizes the action on every topic.
func (a *mercureAuthz) grantsAll(tss *TopicSelectorStore, action mercureAction, topics []string) bool {
	for _, t := range topics {
		if !a.grants(tss, action, t) {
			return false
		}
	}

	return true
}

// subscribeMatchers returns every topic matcher carried by a subscribe detail,
// used as the subscriber's allowed private matchers.
func (a *mercureAuthz) subscribeMatchers() []topicMatcher {
	if a == nil {
		return nil
	}

	var matchers []topicMatcher //nolint:prealloc

	for i := range a.details {
		if !a.details[i].subscribe {
			continue
		}

		matchers = append(matchers, a.details[i].topics...)
	}

	return matchers
}

// subscribePayload returns the payload of the first subscribe detail whose
// topics match the subscription's own matcher m (the `*` wildcard matches
// every subscription). The boolean reports whether a matching detail was
// found, regardless of whether it carried a payload.
func (a *mercureAuthz) subscribePayload(tss *TopicSelectorStore, m topicMatcher) (any, bool) {
	if a == nil {
		return nil, false
	}

	pattern := []string{m.Pattern}

	for i := range a.details {
		if !a.details[i].subscribe {
			continue
		}

		for _, tm := range a.details[i].topics {
			if tm.Pattern == "*" || tss.matchMatcher(pattern, tm) {
				return a.details[i].payload, true
			}
		}
	}

	return nil, false
}

func (d *validatedDetail) hasAction(action mercureAction) bool {
	switch action {
	case actionPublish:
		return d.publish
	case actionSubscribe:
		return d.subscribe
	default:
		return false
	}
}
