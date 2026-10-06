package mercure

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTSS(tb testing.TB) *TopicMatcherStore {
	tb.Helper()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(tb, err)

	return tms
}

func TestDetailTopicUnmarshal(t *testing.T) {
	var d detailTopic
	require.NoError(t, json.Unmarshal([]byte(`{"match":"https://example.com/foo"}`), &d))
	assert.Equal(t, MatcherTypeExact, d.Type)
	assert.Equal(t, "https://example.com/foo", d.Pattern)

	require.NoError(t, json.Unmarshal([]byte(`{"match":"/books/:id","match_type":"urlpattern"}`), &d))
	assert.Equal(t, MatcherTypeURLPattern, d.Type)

	// Bare strings (the deprecated claim shape) are rejected.
	require.ErrorIs(t, json.Unmarshal([]byte(`"https://example.com/foo"`), &d), errInvalidAuthorizationDetail)
}

// TestAuthorizationDetailsRejectAmbiguousJSON covers the two ways a JSON
// document can be read differently by two parsers: a duplicate object member
// (RFC 8259 leaves the outcome unspecified, and v1 silently kept the last one)
// and invalid UTF-8. An authorization server that validates the first
// occurrence while the hub honours the last would grant topics nobody
// approved, so the hub rejects both rather than picking a winner.
func TestAuthorizationDetailsRejectAmbiguousJSON(t *testing.T) {
	t.Parallel()

	for name, claim := range map[string]string{
		"duplicate member in topic entry": `{"authorization_details":[{"type":"` + authorizationDetailTypeMercure + `","actions":["subscribe"],"topics":[{"match":"https://example.com/a","match":"https://example.com/b"}]}]}`,
		"duplicate member in detail":      `{"authorization_details":[{"type":"` + authorizationDetailTypeMercure + `","actions":["subscribe"],"actions":["publish"],"topics":[{"match":"https://example.com/a"}]}]}`,
		"duplicate match_type":            `{"authorization_details":[{"type":"` + authorizationDetailTypeMercure + `","actions":["subscribe"],"topics":[{"match":"https://example.com/a","match_type":"exact","match_type":"urlpattern"}]}]}`,
		"invalid UTF-8 in match":          "{\"authorization_details\":[{\"type\":\"" + authorizationDetailTypeMercure + "\",\"actions\":[\"subscribe\"],\"topics\":[{\"match\":\"https://example.com/a\xffb\"}]}]}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var c claims
			require.Error(t, json.Unmarshal([]byte(claim), &c))
		})
	}
}

// TestAuthorizationDetailsAcceptsWellFormedClaim guards the rejections above
// against over-reach: the same decode path must still accept a valid claim,
// including the unknown members RFC 9396 requires issuers be free to add.
func TestAuthorizationDetailsAcceptsWellFormedClaim(t *testing.T) {
	t.Parallel()

	var c claims
	require.NoError(t, json.Unmarshal([]byte(`{"authorization_details":[
		{"type":"payment_initiation","topics":"not-a-mercure-shape"},
		{"type":"`+authorizationDetailTypeMercure+`","actions":["subscribe"],"topics":[{"match":"https://example.com/a"}],"unknown":1}
	]}`), &c))

	require.Len(t, c.AuthorizationDetails, 2)
	assert.Equal(t, authorizationDetailTypeMercure, c.AuthorizationDetails[1].Type)
	assert.Equal(t, "https://example.com/a", c.AuthorizationDetails[1].Topics[0].Pattern)
}

func TestValidateAuthorizationDetails(t *testing.T) {
	tms := newTestTSS(t)

	t.Run("skips non-mercure entries", func(t *testing.T) {
		authz, err := validateAuthorizationDetails(tms, []authorizationDetail{
			{Type: "payment_initiation"},
		})
		require.NoError(t, err)
		assert.Empty(t, authz.details)
	})

	t.Run("valid detail", func(t *testing.T) {
		authz, err := validateAuthorizationDetails(tms, []authorizationDetail{{
			Type:    authorizationDetailTypeMercure,
			Actions: []mercureAction{actionSubscribe, actionPublish},
			Topics:  []detailTopic{{TopicMatcher{MatcherTypeExact, "https://example.com/foo"}}},
		}})
		require.NoError(t, err)
		require.Len(t, authz.details, 1)
		assert.True(t, authz.details[0].publish)
		assert.True(t, authz.details[0].subscribe)
	})

	t.Run("ignores unrecognized actions", func(t *testing.T) {
		authz, err := validateAuthorizationDetails(tms, []authorizationDetail{{
			Type:    authorizationDetailTypeMercure,
			Actions: []mercureAction{"delete", actionSubscribe},
			Topics:  []detailTopic{{TopicMatcher{MatcherTypeExact, "https://example.com/foo"}}},
		}})
		require.NoError(t, err)
		require.Len(t, authz.details, 1)
		assert.False(t, authz.details[0].publish)
		assert.True(t, authz.details[0].subscribe)
	})

	t.Run("detail with only unrecognized actions grants nothing", func(t *testing.T) {
		authz, err := validateAuthorizationDetails(tms, []authorizationDetail{{
			Type:    authorizationDetailTypeMercure,
			Actions: []mercureAction{"delete"},
			Topics:  []detailTopic{{TopicMatcher{MatcherTypeExact, "https://example.com/foo"}}},
		}})
		require.NoError(t, err)
		require.Len(t, authz.details, 1)
		assert.False(t, authz.details[0].publish)
		assert.False(t, authz.details[0].subscribe)
	})

	for name, tc := range map[string]authorizationDetail{
		"empty actions": {Type: authorizationDetailTypeMercure, Topics: []detailTopic{{TopicMatcher{MatcherTypeExact, "a"}}}},
		"empty topics":  {Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish}},
		"unknown match_type": {
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
			Topics: []detailTopic{{TopicMatcher{"Regexp", "a"}}},
		},
		"forged deprecated type": {
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
			Topics: []detailTopic{{TopicMatcher{deprecatedMatcherTypeName, "a"}}},
		},
		"invalid url pattern": {
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics: []detailTopic{{TopicMatcher{MatcherTypeURLPattern, "https://example.com/[("}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := validateAuthorizationDetails(tms, []authorizationDetail{tc})
			require.ErrorIs(t, err, errInvalidAuthorizationDetail)
		})
	}

	t.Run("too many details", func(t *testing.T) {
		details := make([]authorizationDetail, maxMercureDetails+1)
		for i := range details {
			details[i] = authorizationDetail{
				Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
				Topics: []detailTopic{{TopicMatcher{MatcherTypeExact, "a"}}},
			}
		}

		_, err := validateAuthorizationDetails(tms, details)
		require.ErrorIs(t, err, errInvalidAuthorizationDetail)
	})

	t.Run("too many topics", func(t *testing.T) {
		topics := make([]detailTopic, maxDetailTopics+1)
		for i := range topics {
			topics[i] = detailTopic{TopicMatcher{MatcherTypeExact, "a"}}
		}

		_, err := validateAuthorizationDetails(tms, []authorizationDetail{{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish}, Topics: topics,
		}})
		require.ErrorIs(t, err, errInvalidAuthorizationDetail)
	})

	// The cap is cumulative across details: two details each within the limit
	// but over it combined are rejected, so the matcher count (and thus URL
	// Pattern compilation work) cannot be split to bypass the bound.
	t.Run("too many topics across details", func(t *testing.T) {
		half := maxDetailTopics/2 + 1
		mk := func() authorizationDetail {
			topics := make([]detailTopic, half)
			for i := range topics {
				topics[i] = detailTopic{TopicMatcher{MatcherTypeExact, "a"}}
			}

			return authorizationDetail{Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish}, Topics: topics}
		}

		_, err := validateAuthorizationDetails(tms, []authorizationDetail{mk(), mk()})
		require.ErrorIs(t, err, errInvalidAuthorizationDetail)
	})
}

func TestMercureAuthzGrants(t *testing.T) {
	tms := newTestTSS(t)

	authz, err := validateAuthorizationDetails(tms, []authorizationDetail{
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
			Topics: []detailTopic{{TopicMatcher{MatcherTypeExact, "https://example.com/pub"}}},
		},
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics: []detailTopic{{TopicMatcher{MatcherTypeURLPattern, "https://example.com/books/:id"}}},
		},
	})
	require.NoError(t, err)

	assert.True(t, authz.grants(tms, actionPublish, "https://example.com/pub"))
	assert.False(t, authz.grants(tms, actionSubscribe, "https://example.com/pub"))
	assert.True(t, authz.grants(tms, actionSubscribe, "https://example.com/books/42"))
	assert.False(t, authz.grants(tms, actionPublish, "https://example.com/books/42"))

	assert.True(t, authz.grantsAll(tms, actionSubscribe, []string{"https://example.com/books/1", "https://example.com/books/2"}))
	assert.False(t, authz.grantsAll(tms, actionSubscribe, []string{"https://example.com/books/1", "https://example.com/other"}))

	// nil receiver grants nothing.
	var nilAuthz *mercureAuthz
	assert.False(t, nilAuthz.grants(tms, actionPublish, "x"))
}

func TestMercureAuthzWildcard(t *testing.T) {
	tms := newTestTSS(t)

	authz, err := validateAuthorizationDetails(tms, []authorizationDetail{{
		Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish, actionSubscribe},
		Topics: []detailTopic{{TopicMatcher{MatcherTypeExact, "*"}}},
	}})
	require.NoError(t, err)

	assert.True(t, authz.grants(tms, actionPublish, "anything"))
	assert.True(t, authz.grants(tms, actionSubscribe, "https://example.com/x"))
}

func TestMercureAuthzSubscribePayload(t *testing.T) {
	tms := newTestTSS(t)

	authz, err := validateAuthorizationDetails(tms, []authorizationDetail{
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics:  []detailTopic{{TopicMatcher{MatcherTypeExact, "https://example.com/foo"}}},
			Payload: map[string]any{"k": "specific"},
		},
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics:  []detailTopic{{TopicMatcher{MatcherTypeExact, "*"}}},
			Payload: map[string]any{"k": "default"},
		},
	})
	require.NoError(t, err)

	p, ok := authz.subscribePayload(tms, TopicMatcher{MatcherTypeExact, "https://example.com/foo"})
	require.True(t, ok)
	assert.Equal(t, map[string]any{"k": "specific"}, p)

	// Falls through to the wildcard default.
	p, ok = authz.subscribePayload(tms, TopicMatcher{MatcherTypeExact, "https://example.com/other"})
	require.True(t, ok)
	assert.Equal(t, map[string]any{"k": "default"}, p)

	// Matchers carrying every subscribe topic.
	assert.Len(t, authz.subscribeMatchers(), 2)
}
