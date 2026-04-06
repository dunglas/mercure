package mercure

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionsHandlerAccessDenied(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, subscriptionsURL, nil)
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, subscriptionsURL, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})

	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/bar", nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})

	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())
}

func TestSubscriptionHandlerAccessDenied(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/bar/baz", nil)
	w := httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/bar/baz", nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})

	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())
}

func TestSubscriptionHandlersETag(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	req.Header.Add("If-None-Match", EarliestLastEventID)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusNotModified, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/foo/bar", nil)
	req.Header.Add("If-None-Match", EarliestLastEventID)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo/bar"})})

	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusNotModified, res.StatusCode)
	require.NoError(t, res.Body.Close())
}

func TestSubscriptionsHandler(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	tss := &TopicSelectorStore{}
	logger := slog.Default()
	ctx := t.Context()

	s1 := NewLocalSubscriber("", logger, tss)
	s1.setMatchers(stringsToLegacyMatchers([]string{"https://example.com/foo"}), stringsToLegacyMatchers(nil))
	require.NoError(t, hub.transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", logger, tss)
	s2.setMatchers(stringsToLegacyMatchers([]string{"https://example.com/bar"}), stringsToLegacyMatchers(nil))
	require.NoError(t, hub.transport.AddSubscriber(ctx, s2))

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var subscriptions subscriptionCollection
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscriptions))

	assert.Equal(t, "https://mercure.rocks/", subscriptions.Context)
	assert.Equal(t, subscriptionsURL, subscriptions.ID)
	assert.Equal(t, "Subscriptions", subscriptions.Type)

	lastEventID, subscribers, _ := hub.transport.(TransportSubscribers).GetSubscribers(t.Context())

	assert.Equal(t, lastEventID, subscriptions.LastEventID)
	require.NotEmpty(t, subscribers)

	for _, s := range subscribers {
		currentSubs := s.getSubscriptions(subscriptionFilter{}, "", true)
		require.NotEmpty(t, currentSubs)

		for _, sub := range currentSubs {
			assert.Contains(t, subscriptions.Subscriptions, sub)
		}
	}
}

func TestSubscriptionsHandlerForTopic(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	tss := &TopicSelectorStore{}
	ctx := t.Context()
	logger := slog.Default()

	s1 := NewLocalSubscriber("", logger, tss)
	s1.setMatchers(stringsToLegacyMatchers([]string{"https://example.com/foo"}), stringsToLegacyMatchers(nil))
	require.NoError(t, hub.transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", logger, tss)
	s2.setMatchers(stringsToLegacyMatchers([]string{"https://example.com/bar"}), stringsToLegacyMatchers(nil))
	require.NoError(t, hub.transport.AddSubscriber(ctx, s2))

	escapedBarTopic := url.QueryEscape("https://example.com/bar")

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionsForTopicURL, hub.SubscriptionsHandler)

	s2EscapedTopic := s2.EscapedMatchers[0].EscapedPattern

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/"+s2EscapedTopic, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/" + s2EscapedTopic})})

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var subscriptions subscriptionCollection
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscriptions))

	assert.Equal(t, "https://mercure.rocks/", subscriptions.Context)
	assert.Equal(t, defaultHubURL+subscriptionsPath+"/"+escapedBarTopic, subscriptions.ID)
	assert.Equal(t, "Subscriptions", subscriptions.Type)

	lastEventID, subscribers, _ := hub.transport.(TransportSubscribers).GetSubscribers(t.Context())

	assert.Equal(t, lastEventID, subscriptions.LastEventID)
	require.NotEmpty(t, subscribers)

	for _, s := range subscribers {
		for _, sub := range s.getSubscriptions(subscriptionFilter{topic: "https://example.com/bar"}, "", true) {
			require.NotContains(t, "foo", sub.Topic)
			assert.Contains(t, subscriptions.Subscriptions, sub)
		}
	}
}

func TestSubscriptionHandler(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	tss := &TopicSelectorStore{}
	ctx := t.Context()
	logger := slog.Default()

	otherS := NewLocalSubscriber("", logger, tss)
	otherS.setMatchers(stringsToLegacyMatchers([]string{"https://example.com/other"}), stringsToLegacyMatchers(nil))
	require.NoError(t, hub.transport.AddSubscriber(ctx, otherS))

	sTopics := []string{"https://example.com/other", "https://example.com/{foo}"}
	s := NewLocalSubscriber("", logger, tss)
	s.setMatchers(stringsToLegacyMatchers(sTopics), stringsToLegacyMatchers(nil))
	require.NoError(t, hub.transport.AddSubscriber(ctx, s))

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionURL, hub.SubscriptionHandler)

	sEscapedTemplate := s.EscapedMatchers[1].EscapedPattern

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/"+sEscapedTemplate+"/"+s.EscapedID, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var subscription subscription
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscription))

	expectedSub := s.getSubscriptions(subscriptionFilter{topic: sTopics[1]}, "https://mercure.rocks/", true)[0]
	expectedSub.LastEventID, _, _ = hub.transport.(TransportSubscribers).GetSubscribers(t.Context())
	assert.Equal(t, expectedSub, subscription)

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/notexist/"+s.EscapedID, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	require.NoError(t, res.Body.Close())
}

// TestSubscriptionPayloadFromMatchingClaim verifies the spec rule: the payload
// attached to a subscription is the payload of the FIRST JWT subscribe-claim
// that matches the subscription's own matcher — not the claim at the same
// positional index.
func TestSubscriptionPayloadFromMatchingClaim(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, withAllMatcherTypes()...)
	logger := slog.Default()

	sub := NewLocalSubscriber("", logger, hub.topicSelectorStore)
	matchers, err := hub.parseMatchers(url.Values{
		"match": {"https://example.com/foo", "https://example.com/bar"},
	}, false)
	require.NoError(t, err)

	sub.Claims = &claims{
		Mercure: mercureClaim{
			Subscribe: []matcherClaim{
				// Non-matching claim first — must not be picked.
				{topicMatcher: topicMatcher{Type: "exact", Pattern: "https://other.example.com/x"}, Payload: map[string]any{"tag": "x"}},
				// This URLPattern claim covers /foo AND /bar → gets picked as "first matching".
				{topicMatcher: topicMatcher{Type: "urlpattern", Pattern: "https://example.com/:id"}, Payload: map[string]any{"tag": "urlpattern"}},
				// Exact claim for /bar — would only win if iteration reached it first.
				{topicMatcher: topicMatcher{Type: "exact", Pattern: "https://example.com/bar"}, Payload: map[string]any{"tag": "exact-bar"}},
			},
		},
	}
	require.NoError(t, resolveMatcherClaims(hub.topicSelectorStore, sub.Claims.Mercure.Subscribe, false))

	sub.setMatchers(matchers, nil)

	subs := sub.getSubscriptions(subscriptionFilter{}, "", true)
	require.Len(t, subs, 2)

	for _, s := range subs {
		p, ok := s.Payload.(map[string]any)
		require.True(t, ok, "payload must come from a matching claim")
		assert.Equal(t, "urlpattern", p["tag"], "first MATCHING claim wins, not the first claim by index")
	}
}

// TestSubscriptionPayloadFallbackToGlobal verifies the fallback to mercure.payload
// when no per-claim payload matches the subscription's matcher.
func TestSubscriptionPayloadFallbackToGlobal(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, withAllMatcherTypes()...)
	logger := slog.Default()

	sub := NewLocalSubscriber("", logger, hub.topicSelectorStore)
	matchers, err := hub.parseMatchers(url.Values{
		"match": {"https://example.com/foo"},
	}, false)
	require.NoError(t, err)

	sub.Claims = &claims{
		Mercure: mercureClaim{
			Payload: map[string]any{"global": true},
			Subscribe: []matcherClaim{
				// A claim that doesn't match the subscription's matcher.
				{topicMatcher: topicMatcher{Type: "exact", Pattern: "https://other.example.com/x"}, Payload: map[string]any{"tag": "ignored"}},
			},
		},
	}
	require.NoError(t, resolveMatcherClaims(hub.topicSelectorStore, sub.Claims.Mercure.Subscribe, false))

	sub.setMatchers(matchers, nil)

	subs := sub.getSubscriptions(subscriptionFilter{}, "", true)
	require.Len(t, subs, 1)

	p, ok := subs[0].Payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, p["global"])
}

// TestSubscriptionHandlerMatchRoute exercises the new
// /subscriptions/{matchType}/{match}/{subscriber} URL shape.
func TestSubscriptionHandlerMatchRoute(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, withAllMatcherTypes()...)
	ctx := t.Context()
	logger := slog.Default()

	sub := NewLocalSubscriber("", logger, hub.topicSelectorStore)
	matchers, err := hub.parseMatchers(url.Values{
		"matchURLPattern": {"https://example.com/:id"},
	}, false)
	require.NoError(t, err)
	sub.setMatchers(matchers, nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, sub))

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionMatchURL, hub.SubscriptionHandler)

	// Use the escaped matcher directly from the subscriber to avoid encoding drift.
	em := sub.EscapedMatchers[0]
	authURL := "/.well-known/mercure/subscriptions/" + em.EscapedType + "/" + em.EscapedPattern + "/" + sub.EscapedID
	reqURL := authURL

	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{authURL})})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var got subscription
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	assert.Equal(t, "https://example.com/:id", got.Match)
	assert.Equal(t, "URLPattern", got.MatchType)
	assert.Empty(t, got.Topic, "new-style subscription must not emit the legacy `topic` field")
}
