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
	hub := createDummy()

	req := httptest.NewRequest("GET", subscriptionsURL, nil)
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	res.Body.Close()

	req = httptest.NewRequest("GET", subscriptionsURL, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})
	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	res.Body.Close()

	req = httptest.NewRequest("GET", defaultHubURL+"/subscriptions/bar", nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})
	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	res.Body.Close()
}

func TestSubscriptionHandlerAccessDenied(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", defaultHubURL+"/subscriptions/bar/baz", nil)
	w := httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	res.Body.Close()

	req = httptest.NewRequest("GET", defaultHubURL+"/subscriptions/bar/baz", nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})
	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	res.Body.Close()
}

func TestSubscriptionHandlersETag(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", defaultHubURL+"/subscriptions", nil)
	req.Header.Add("If-None-Match", EarliestLastEventID)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusNotModified, res.StatusCode)
	res.Body.Close()

	req = httptest.NewRequest("GET", defaultHubURL+"/subscriptions/foo/bar", nil)
	req.Header.Add("If-None-Match", EarliestLastEventID)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo/bar"})})
	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusNotModified, res.StatusCode)
	res.Body.Close()
}

func TestSubscriptionsHandler(t *testing.T) {
	hub := createDummy()

	s1 := NewSubscriber("", zap.NewNop(), hub.topicSelectorStore)
	s1.Topics = []string{"http://example.com/foo"}
	s1.EscapedTopics = []string{url.QueryEscape(s1.Topics[0])}
	go s1.start()
	require.Nil(t, hub.transport.AddSubscriber(s1))

	s2 := NewSubscriber("", zap.NewNop(), hub.topicSelectorStore)
	s2.Topics = []string{"http://example.com/bar"}
	s2.EscapedTopics = []string{url.QueryEscape(s2.Topics[0])}
	go s2.start()
	require.Nil(t, hub.transport.AddSubscriber(s2))

	req := httptest.NewRequest("GET", defaultHubURL+"/subscriptions", nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	res.Body.Close()

	var subscriptions subscriptionCollection
	json.Unmarshal(w.Body.Bytes(), &subscriptions)

	assert.Equal(t, "https://mercure.rocks/", subscriptions.Context)
	assert.Equal(t, subscriptionsURL, subscriptions.ID)
	assert.Equal(t, "Subscriptions", subscriptions.Type)

	lastEventID, subscribers := hub.transport.(TransportSubscribers).GetSubscribers()

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
	hub := createDummy()

	s1 := NewSubscriber("", zap.NewNop(), hub.topicSelectorStore)
	s1.Topics = []string{"http://example.com/foo"}
	s1.EscapedTopics = []string{url.QueryEscape(s1.Topics[0])}
	go s1.start()
	require.Nil(t, hub.transport.AddSubscriber(s1))

	s2 := NewSubscriber("", zap.NewNop(), hub.topicSelectorStore)
	s2.Topics = []string{"http://example.com/bar"}
	s2.EscapedTopics = []string{url.QueryEscape(s2.Topics[0])}
	go s2.start()
	require.Nil(t, hub.transport.AddSubscriber(s2))

	escapedBarTopic := url.QueryEscape("http://example.com/bar")

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionsForTopicURL, hub.SubscriptionsHandler)

	req := httptest.NewRequest("GET", defaultHubURL+"/subscriptions/"+s2.EscapedTopics[0], nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/" + s2.EscapedTopics[0]})})
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	res.Body.Close()

	var subscriptions subscriptionCollection
	json.Unmarshal(w.Body.Bytes(), &subscriptions)

	assert.Equal(t, "https://mercure.rocks/", subscriptions.Context)
	assert.Equal(t, defaultHubURL+"/subscriptions/"+escapedBarTopic, subscriptions.ID)
	assert.Equal(t, "Subscriptions", subscriptions.Type)

	lastEventID, subscribers := hub.transport.(TransportSubscribers).GetSubscribers()

	assert.Equal(t, lastEventID, subscriptions.LastEventID)
	require.NotEmpty(t, subscribers)
	for _, s := range subscribers {
		for _, sub := range s.getSubscriptions("http://example.com/bar", "", true) {
			require.NotContains(t, "foo", sub.Topic)
			assert.Contains(t, subscriptions.Subscriptions, sub)
		}
	}
}

func TestSubscriptionHandler(t *testing.T) {
	hub := createDummy()

	otherS := NewSubscriber("", zap.NewNop(), hub.topicSelectorStore)
	otherS.Topics = []string{"http://example.com/other"}
	otherS.EscapedTopics = []string{url.QueryEscape(otherS.Topics[0])}
	go otherS.start()
	require.Nil(t, hub.transport.AddSubscriber(otherS))

	s := NewSubscriber("", zap.NewNop(), hub.topicSelectorStore)
	s.Topics = []string{"http://example.com/other", "http://example.com/{foo}"}
	s.EscapedTopics = []string{url.QueryEscape(s.Topics[0]), url.QueryEscape(s.Topics[1])}
	go s.start()
	require.Nil(t, hub.transport.AddSubscriber(s))

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionURL, hub.SubscriptionHandler)

	req := httptest.NewRequest("GET", defaultHubURL+"/subscriptions/"+s.EscapedTopics[1]+"/"+s.EscapedID, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	res.Body.Close()

	var subscription subscription
	json.Unmarshal(w.Body.Bytes(), &subscription)
	expectedSub := s.getSubscriptions(s.Topics[1], "https://mercure.rocks/", true)[1]
	expectedSub.LastEventID, _ = hub.transport.(TransportSubscribers).GetSubscribers()
	assert.Equal(t, expectedSub, subscription)

	req = httptest.NewRequest("GET", defaultHubURL+"/subscriptions/notexist/"+s.EscapedID, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	res.Body.Close()
}
