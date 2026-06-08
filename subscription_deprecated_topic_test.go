//go:build deprecated_topic && deprecated_claim

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

// TestSubscriptionsHandlerForTopic exercises the deprecated
// /subscriptions/{topic} route (only registered under
// WithProtocolVersionCompatibility(8)).
func TestSubscriptionsHandlerForTopic(t *testing.T) {
	t.Parallel()

	hub := createDeprecatedDummy(t)
	tss := &TopicSelectorStore{}
	ctx := t.Context()
	logger := slog.Default()

	s1 := NewLocalSubscriber("", logger, tss)
	s1.setMatchers(stringsToDeprecatedMatchers([]string{"https://example.com/foo"}), nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", logger, tss)
	s2.setMatchers(stringsToDeprecatedMatchers([]string{"https://example.com/bar"}), nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, s2))

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionsForTopicURL, hub.SubscriptionsHandler)

	s2EscapedTopic := url.QueryEscape(s2.SubscribedMatchers[0].Pattern)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/"+s2EscapedTopic, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDeprecatedAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/" + s2EscapedTopic})})

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var subscriptions subscriptionCollection
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscriptions))

	assert.Equal(t, "https://mercure.rocks/", subscriptions.Context)
	assert.Equal(t, defaultHubURL+subscriptionsPath+"/"+s2EscapedTopic, subscriptions.ID)
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

// TestSubscriptionHandlerForTopic exercises the deprecated
// /subscriptions/{topic}/{subscriber} route.
func TestSubscriptionHandlerForTopic(t *testing.T) {
	t.Parallel()

	hub := createDeprecatedDummy(t)
	tss := &TopicSelectorStore{}
	ctx := t.Context()
	logger := slog.Default()

	otherS := NewLocalSubscriber("", logger, tss)
	otherS.setMatchers(stringsToDeprecatedMatchers([]string{"https://example.com/other"}), nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, otherS))

	sTopics := []string{"https://example.com/other", "https://example.com/{foo}"}
	s := NewLocalSubscriber("", logger, tss)
	s.setMatchers(stringsToDeprecatedMatchers(sTopics), nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, s))

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionURL, hub.SubscriptionHandler)

	sEscapedTemplate := url.QueryEscape(s.SubscribedMatchers[1].Pattern)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/"+sEscapedTemplate+"/"+s.EscapedID, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDeprecatedAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})

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
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDeprecatedAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions{/topic}{/subscriber}"})})

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	require.NoError(t, res.Body.Close())
}
