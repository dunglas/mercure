package mercure

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTSS(tb testing.TB) *TopicSelectorStore {
	tb.Helper()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(tb, err)

	return tss
}

func TestDetailTopicUnmarshal(t *testing.T) {
	var d detailTopic
	require.NoError(t, json.Unmarshal([]byte(`{"match":"https://example.com/foo"}`), &d))
	assert.Equal(t, MatcherTypeExact, d.Type)
	assert.Equal(t, "https://example.com/foo", d.Pattern)

	require.NoError(t, json.Unmarshal([]byte(`{"match":"/books/:id","matchType":"URLPattern"}`), &d))
	assert.Equal(t, MatcherTypeURLPattern, d.Type)

	// Bare strings (the deprecated claim shape) are rejected.
	require.ErrorIs(t, json.Unmarshal([]byte(`"https://example.com/foo"`), &d), errInvalidAuthorizationDetail)
}

func TestValidateAuthorizationDetails(t *testing.T) {
	tss := newTestTSS(t)

	t.Run("skips non-mercure entries", func(t *testing.T) {
		authz, err := validateAuthorizationDetails(tss, []authorizationDetail{
			{Type: "payment_initiation"},
		})
		require.NoError(t, err)
		assert.Empty(t, authz.details)
	})

	t.Run("valid detail", func(t *testing.T) {
		authz, err := validateAuthorizationDetails(tss, []authorizationDetail{{
			Type:    authorizationDetailTypeMercure,
			Actions: []mercureAction{actionSubscribe, actionPublish},
			Topics:  []detailTopic{{topicMatcher{MatcherTypeExact, "https://example.com/foo"}}},
		}})
		require.NoError(t, err)
		require.Len(t, authz.details, 1)
		assert.True(t, authz.details[0].publish)
		assert.True(t, authz.details[0].subscribe)
	})

	for name, tc := range map[string]authorizationDetail{
		"empty actions":  {Type: authorizationDetailTypeMercure, Topics: []detailTopic{{topicMatcher{MatcherTypeExact, "a"}}}},
		"unknown action": {Type: authorizationDetailTypeMercure, Actions: []mercureAction{"delete"}, Topics: []detailTopic{{topicMatcher{MatcherTypeExact, "a"}}}},
		"empty topics":   {Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish}},
		"unknown matchType": {
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
			Topics: []detailTopic{{topicMatcher{"Regexp", "a"}}},
		},
		"forged deprecated type": {
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
			Topics: []detailTopic{{topicMatcher{deprecatedMatcherTypeName, "a"}}},
		},
		"invalid url pattern": {
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics: []detailTopic{{topicMatcher{MatcherTypeURLPattern, "https://example.com/[("}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := validateAuthorizationDetails(tss, []authorizationDetail{tc})
			require.ErrorIs(t, err, errInvalidAuthorizationDetail)
		})
	}

	t.Run("too many details", func(t *testing.T) {
		details := make([]authorizationDetail, maxMercureDetails+1)
		for i := range details {
			details[i] = authorizationDetail{
				Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
				Topics: []detailTopic{{topicMatcher{MatcherTypeExact, "a"}}},
			}
		}

		_, err := validateAuthorizationDetails(tss, details)
		require.ErrorIs(t, err, errInvalidAuthorizationDetail)
	})

	t.Run("too many topics", func(t *testing.T) {
		topics := make([]detailTopic, maxDetailTopics+1)
		for i := range topics {
			topics[i] = detailTopic{topicMatcher{MatcherTypeExact, "a"}}
		}

		_, err := validateAuthorizationDetails(tss, []authorizationDetail{{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish}, Topics: topics,
		}})
		require.ErrorIs(t, err, errInvalidAuthorizationDetail)
	})
}

func TestMercureAuthzGrants(t *testing.T) {
	tss := newTestTSS(t)

	authz, err := validateAuthorizationDetails(tss, []authorizationDetail{
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish},
			Topics: []detailTopic{{topicMatcher{MatcherTypeExact, "https://example.com/pub"}}},
		},
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics: []detailTopic{{topicMatcher{MatcherTypeURLPattern, "https://example.com/books/:id"}}},
		},
	})
	require.NoError(t, err)

	assert.True(t, authz.grants(tss, actionPublish, "https://example.com/pub"))
	assert.False(t, authz.grants(tss, actionSubscribe, "https://example.com/pub"))
	assert.True(t, authz.grants(tss, actionSubscribe, "https://example.com/books/42"))
	assert.False(t, authz.grants(tss, actionPublish, "https://example.com/books/42"))

	assert.True(t, authz.grantsAll(tss, actionSubscribe, []string{"https://example.com/books/1", "https://example.com/books/2"}))
	assert.False(t, authz.grantsAll(tss, actionSubscribe, []string{"https://example.com/books/1", "https://example.com/other"}))

	// nil receiver grants nothing.
	var nilAuthz *mercureAuthz
	assert.False(t, nilAuthz.grants(tss, actionPublish, "x"))
}

func TestMercureAuthzWildcard(t *testing.T) {
	tss := newTestTSS(t)

	authz, err := validateAuthorizationDetails(tss, []authorizationDetail{{
		Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionPublish, actionSubscribe},
		Topics: []detailTopic{{topicMatcher{MatcherTypeExact, "*"}}},
	}})
	require.NoError(t, err)

	assert.True(t, authz.grants(tss, actionPublish, "anything"))
	assert.True(t, authz.grants(tss, actionSubscribe, "https://example.com/x"))
}

func TestMercureAuthzSubscribePayload(t *testing.T) {
	tss := newTestTSS(t)

	authz, err := validateAuthorizationDetails(tss, []authorizationDetail{
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics:  []detailTopic{{topicMatcher{MatcherTypeExact, "https://example.com/foo"}}},
			Payload: map[string]any{"k": "specific"},
		},
		{
			Type: authorizationDetailTypeMercure, Actions: []mercureAction{actionSubscribe},
			Topics:  []detailTopic{{topicMatcher{MatcherTypeExact, "*"}}},
			Payload: map[string]any{"k": "default"},
		},
	})
	require.NoError(t, err)

	p, ok := authz.subscribePayload(tss, topicMatcher{MatcherTypeExact, "https://example.com/foo"})
	require.True(t, ok)
	assert.Equal(t, map[string]any{"k": "specific"}, p)

	// Falls through to the wildcard default.
	p, ok = authz.subscribePayload(tss, topicMatcher{MatcherTypeExact, "https://example.com/other"})
	require.True(t, ok)
	assert.Equal(t, map[string]any{"k": "default"}, p)

	// Matchers carrying every subscribe topic.
	assert.Len(t, authz.subscribeMatchers(), 2)
}
