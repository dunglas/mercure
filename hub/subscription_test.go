package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscriptionsHandlerAccessDenied(t *testing.T) {
	hub := createDummy()
	hub.config.Set("subscriptions", true)

	req := httptest.NewRequest("GET", subscriptionsURL, nil)
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)

	req = httptest.NewRequest("GET", subscriptionsURL, nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})
	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)

	req = httptest.NewRequest("GET", defaultHubURL+"/subscriptions/bar", nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})
	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestSubscriptionHandlerAccessDenied(t *testing.T) {
	hub := createDummy()
	hub.config.Set("subscriptions", true)

	req := httptest.NewRequest("GET", defaultHubURL+"/subscriptions/bar/baz", nil)
	w := httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)

	req = httptest.NewRequest("GET", defaultHubURL+"/subscriptions/bar/baz", nil)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo{/subscriber}"})})
	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestSubscriptionHandlersETag(t *testing.T) {
	hub := createDummy()
	hub.config.Set("subscriptions", true)

	req := httptest.NewRequest("GET", defaultHubURL+"/subscriptions", nil)
	req.Header.Add("If-None-Match", EarliestLastEventID)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	assert.Equal(t, http.StatusNotModified, w.Result().StatusCode)

	req = httptest.NewRequest("GET", defaultHubURL+"/subscriptions/foo/bar", nil)
	req.Header.Add("If-None-Match", EarliestLastEventID)
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo/bar"})})
	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	assert.Equal(t, http.StatusNotModified, w.Result().StatusCode)
}
