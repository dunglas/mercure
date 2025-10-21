package mercure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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

	logger := zap.NewNop()

	hub := createDummy(t, WithLogger(logger))
	tss := &TopicSelectorStore{}

	s1 := NewLocalSubscriber("", logger, tss)
	s1.SetTopics([]string{"https://example.com/foo"}, nil)
	require.NoError(t, hub.transport.AddSubscriber(s1))

	s2 := NewLocalSubscriber("", logger, tss)
	s2.SetTopics([]string{"https://example.com/bar"}, nil)
	require.NoError(t, hub.transport.AddSubscriber(s2))

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

	lastEventID, subscribers, _ := hub.transport.(TransportSubscribers).GetSubscribers()

	assert.Equal(t, lastEventID, subscriptions.LastEventID)
	require.NotEmpty(t, subscribers)

	for _, s := range subscribers {
		currentSubs := s.getSubscriptions("", "", true)
		require.NotEmpty(t, currentSubs)

		for _, sub := range currentSubs {
			assert.Contains(t, subscriptions.Subscriptions, sub)
		}
	}
}

func TestSubscriptionsHandlerForTopic(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	hub := createDummy(t, WithLogger(logger))
	tss := &TopicSelectorStore{}

	s1 := NewLocalSubscriber("", logger, tss)
	s1.SetTopics([]string{"https://example.com/foo"}, nil)
	require.NoError(t, hub.transport.AddSubscriber(s1))

	s2 := NewLocalSubscriber("", logger, tss)
	s2.SetTopics([]string{"https://example.com/bar"}, nil)
	require.NoError(t, hub.transport.AddSubscriber(s2))

	escapedBarTopic := url.QueryEscape("https://example.com/bar")

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionsForTopicURL, hub.SubscriptionsHandler)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/"+s2.EscapedTopics[0], nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/" + s2.EscapedTopics[0]})})

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

	lastEventID, subscribers, _ := hub.transport.(TransportSubscribers).GetSubscribers()

	assert.Equal(t, lastEventID, subscriptions.LastEventID)
	require.NotEmpty(t, subscribers)

	for _, s := range subscribers {
		for _, sub := range s.getSubscriptions("https://example.com/bar", "", true) {
			require.NotContains(t, "foo", sub.Topic)
			assert.Contains(t, subscriptions.Subscriptions, sub)
		}
	}
}

func TestSubscriptionHandler(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	hub := createDummy(t, WithLogger(logger))
	tss := &TopicSelectorStore{}

	otherS := NewLocalSubscriber("", logger, tss)
	otherS.SetTopics([]string{"https://example.com/other"}, nil)
	require.NoError(t, hub.transport.AddSubscriber(otherS))

	s := NewLocalSubscriber("", logger, tss)
	s.SetTopics([]string{"https://example.com/other", "https://example.com/{foo}"}, nil)
	require.NoError(t, hub.transport.AddSubscriber(s))

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionURL, hub.SubscriptionHandler)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/"+s.EscapedTopics[1]+"/"+s.EscapedID, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var subscription subscription
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscription))

	expectedSub := s.getSubscriptions(s.SubscribedTopics[1], "https://mercure.rocks/", true)[0]
	expectedSub.LastEventID, _, _ = hub.transport.(TransportSubscribers).GetSubscribers()
	assert.Equal(t, expectedSub, subscription)

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/notexist/"+s.EscapedID, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	require.NoError(t, res.Body.Close())
}
