package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeInvalidJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: "invalid"})
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/subscribe", nil)
	w := httptest.NewRecorder()
	http.SetCookie(w, &http.Cookie{Name: "mercureAuthorization", Value: createDummyUnauthorizedJWT(hub)})
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestSubscribeNoIRI(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"iri[]\" parameters.\n", w.Body.String())
}
