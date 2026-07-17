package mercure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Caps on the authorization_details claim, aligned with the subscribe query
// limit (maxMatcherCount, in subscribematchers.go): the total number of topic
// matchers a token can carry, summed across all mercure details, is bounded by
// maxMatcherCount, and the number of mercure details by the same value. Each
// matcher can force a URL Pattern compilation during validation, so these
// bounds cap that work to the same ceiling as a subscribe request. A token
// exceeding either is rejected as an invalid token (401).
const (
	maxMercureDetails = maxMatcherCount // type=="mercure" authorization details
	maxDetailTopics   = maxMatcherCount // topic matchers summed across all details
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
// claim is malformed. The HTTP handlers map it to a 401 "invalid_token"
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

// UnmarshalJSON decodes the type of every entry but the mercure-specific
// members only for entries of type "mercure". RFC 9396 lets other detail types
// define their own member shapes (for example a "topics" string), so applying
// the mercure schema to every entry would reject valid multi-resource tokens;
// non-mercure entries are ignored during validation.
func (ad *authorizationDetail) UnmarshalJSON(data []byte) error {
	var head struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, err)
	}

	ad.Type = head.Type
	if head.Type != authorizationDetailTypeMercure {
		return nil
	}

	var body struct {
		Actions []mercureAction `json:"actions"`
		Topics  []detailTopic   `json:"topics"`
		Payload any             `json:"payload"`
	}

	if err := json.Unmarshal(data, &body); err != nil {
		return fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, err)
	}

	ad.Actions = body.Actions
	ad.Topics = body.Topics
	ad.Payload = body.Payload

	return nil
}

// detailTopic is one entry of a mercure authorization detail `topics` array.
// Only the object form {match, match_type?} is accepted; match_type is
// case-sensitive and defaults to Exact.
type detailTopic struct {
	TopicMatcher
}

// MarshalJSON emits the object form {match, match_type}. Issuers normally mint
// tokens, but the hub round-trips them in tests.
func (d detailTopic) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(struct {
		Match     string      `json:"match"`
		MatchType MatcherType `json:"match_type,omitempty"`
	}{d.Pattern, d.Type})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal topic matcher: %w", err)
	}

	return b, nil
}

// UnmarshalJSON enforces the object form with a required "match" property. A
// bare string (the deprecated claim shape) is rejected so that legacy tokens do
// not silently parse as Exact matchers, and an object without "match" (or a
// JSON null) invalidates the token instead of becoming an empty-pattern matcher.
func (d *detailTopic) UnmarshalJSON(data []byte) error {
	// json.Unmarshal(null) is a silent no-op, so reject null explicitly.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf(`%w: a topic entry must be an object with a "match" property`, errInvalidAuthorizationDetail)
	}

	// Match is a pointer so an absent property is distinguishable from an
	// explicit empty string: the protocol requires the property to be present.
	var obj struct {
		Match     *string     `json:"match"`
		MatchType MatcherType `json:"match_type"`
	}

	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("%w: topic entries must be objects: %w", errInvalidAuthorizationDetail, err)
	}

	if obj.Match == nil {
		return fmt.Errorf(`%w: a topic entry is missing the required "match" property`, errInvalidAuthorizationDetail)
	}

	d.Pattern = *obj.Match
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
	topics    []TopicMatcher
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
//
// The caps below bound only mercure entries; the total number of entries the
// token carries (mercure or not) is bounded by the transport's request-size
// limit (Caddy's request_body directive, Go's MaxHeaderBytes), not an explicit
// count here, since the whole claim is decoded before this runs.
func validateAuthorizationDetails(tms *TopicMatcherStore, raw []authorizationDetail) (*mercureAuthz, error) {
	authz := &mercureAuthz{}

	var count, totalTopics int

	for i := range raw {
		if raw[i].Type != authorizationDetailTypeMercure {
			continue
		}

		count++
		if count > maxMercureDetails {
			return nil, fmt.Errorf("%w: too many mercure authorization details (max %d)", errInvalidAuthorizationDetail, maxMercureDetails)
		}

		// Bound the cumulative matcher count before validateMercureDetail
		// compiles any pattern, so a token cannot force unbounded URL Pattern
		// compilation regardless of how the topics are split across details.
		totalTopics += len(raw[i].Topics)
		if totalTopics > maxDetailTopics {
			return nil, fmt.Errorf("%w: too many topics across mercure authorization details (max %d)", errInvalidAuthorizationDetail, maxDetailTopics)
		}

		vd, err := validateMercureDetail(tms, raw[i])
		if err != nil {
			return nil, err
		}

		authz.details = append(authz.details, vd)
	}

	return authz, nil
}

// validateProtocolMatcher enforces the wire constraints on a single topic
// matcher carried by a token: pattern length, allowed characters, a known
// matcher type, and a compilable pattern. It returns the shared sentinels
// (errPatternTooLong, errInvalidMatcherValue, ErrUnsupportedMatcherType) or the
// underlying compiler error; callers wrap the result with their own sentinel.
func validateProtocolMatcher(tms *TopicMatcherStore, m TopicMatcher) error {
	if err := validateMatcherValue(m.Pattern); err != nil {
		return err
	}

	if !knownMatcherType(m.Type) {
		return ErrUnsupportedMatcherType
	}

	return tms.validatePattern(m)
}

// validateMatcherValue enforces the length and character constraints every wire
// matcher pattern shares, independent of matcher type: the single definition of
// those two checks, reused by the query-parameter, JWT-claim and
// authorization-details paths. It returns errPatternTooLong or
// errInvalidMatcherValue.
func validateMatcherValue(pattern string) error {
	if len(pattern) > maxPatternLength {
		return errPatternTooLong
	}

	if !validProtocolString(pattern) {
		return errInvalidMatcherValue
	}

	return nil
}

func validateMercureDetail(tms *TopicMatcherStore, d authorizationDetail) (validatedDetail, error) {
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

	vd.topics = make([]TopicMatcher, len(d.Topics))
	for i := range d.Topics {
		m := d.Topics[i].TopicMatcher

		// An empty pattern is a meaningless matcher; reject it before the shared
		// validator, which does not (an empty Exact pattern compiles fine).
		if m.Pattern == "" {
			return vd, fmt.Errorf("%w: a topic matcher pattern must not be empty", errInvalidAuthorizationDetail)
		}

		// validateProtocolMatcher is the single definition of a valid wire
		// matcher (length, charset, known type, compilable pattern); the
		// internal deprecated type is rejected as an unknown type.
		if err := validateProtocolMatcher(tms, m); err != nil {
			return vd, fmt.Errorf("%w: %w", errInvalidAuthorizationDetail, err)
		}

		vd.topics[i] = m
	}

	return vd, nil
}

// hasPayload reports whether any validated detail carries a payload. It is the
// only case where per-matcher subscription payload resolution can differ from
// the token-wide fallback, so the subscriber uses it to skip the
// O(matchers × details) matcher dispatch when no payload is present.
func (a *mercureAuthz) hasPayload() bool {
	if a == nil {
		return false
	}

	for i := range a.details {
		if a.details[i].payload != nil {
			return true
		}
	}

	return false
}

// grants reports whether the token authorizes the given action on the topic.
func (a *mercureAuthz) grants(tms *TopicMatcherStore, action mercureAction, topic string) bool {
	if a == nil {
		return false
	}

	single := []string{topic}

	for i := range a.details {
		if !a.details[i].hasAction(action) {
			continue
		}

		for _, m := range a.details[i].topics {
			if tms.matchMatcher(single, m) {
				return true
			}
		}
	}

	return false
}

// grantsAll reports whether the token authorizes the action on every topic.
func (a *mercureAuthz) grantsAll(tms *TopicMatcherStore, action mercureAction, topics []string) bool {
	for _, t := range topics {
		if !a.grants(tms, action, t) {
			return false
		}
	}

	return true
}

// subscribeMatchers returns every topic matcher carried by a subscribe detail,
// used as the subscriber's allowed private matchers.
func (a *mercureAuthz) subscribeMatchers() []TopicMatcher {
	if a == nil {
		return nil
	}

	var matchers []TopicMatcher //nolint:prealloc

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
func (a *mercureAuthz) subscribePayload(tms *TopicMatcherStore, m TopicMatcher) (any, bool) {
	if a == nil {
		return nil, false
	}

	pattern := []string{m.Pattern}

	for i := range a.details {
		if !a.details[i].subscribe {
			continue
		}

		for _, tm := range a.details[i].topics {
			// matchMatcher already treats the "*" wildcard as matching every
			// pattern, so no separate wildcard check is needed here.
			if tms.matchMatcher(pattern, tm) {
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
