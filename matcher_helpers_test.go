package mercure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test helpers that wrap a slice of topic strings into Exact-matcher
// topicMatchers. Used by tests that don't specifically exercise the
// deprecated topic path.

func stringsToExactMatchers(patterns []string) []topicMatcher {
	if patterns == nil {
		return nil
	}

	out := make([]topicMatcher, len(patterns))
	for i, p := range patterns {
		out[i] = topicMatcher{Type: MatcherTypeExact, Pattern: p}
	}

	return out
}

// subscribeDetailsFromMatchers builds a single subscribe authorization detail
// covering the given matchers, with an optional payload.
func subscribeDetailsFromMatchers(payload any, matchers ...topicMatcher) []authorizationDetail {
	topics := make([]detailTopic, len(matchers))
	for i, m := range matchers {
		topics[i] = detailTopic{m}
	}

	return []authorizationDetail{{
		Type:    authorizationDetailTypeMercure,
		Actions: []mercureAction{actionSubscribe},
		Topics:  topics,
		Payload: payload,
	}}
}

// createDummySubscriberJWTWithDetails mints a subscriber access token granting
// the subscribe action on the given matchers, carrying the given payload.
func createDummySubscriberJWTWithDetails(tb testing.TB, payload any, matchers ...topicMatcher) string {
	tb.Helper()

	return mintAccessToken([]byte("subscriber"), testResourceIdentifier, subscribeDetailsFromMatchers(payload, matchers...))
}

// subscribeDetail builds a subscribe authorization detail covering a single
// matcher, with an optional payload.
func subscribeDetail(payload any, m topicMatcher) authorizationDetail {
	return authorizationDetail{
		Type:    authorizationDetailTypeMercure,
		Actions: []mercureAction{actionSubscribe},
		Topics:  []detailTopic{{m}},
		Payload: payload,
	}
}

// detailClaims builds a *claims with the given authorization details validated
// into its authz, for tests that set Subscriber.Claims directly.
func detailClaims(tb testing.TB, tss *TopicSelectorStore, details ...authorizationDetail) *claims {
	tb.Helper()

	authz, err := validateAuthorizationDetails(tss, details)
	require.NoError(tb, err)

	return &claims{AuthorizationDetails: details, authz: authz}
}
