package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
)

func TestNoAuthorizationHeader(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestInvalidJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer invalid")
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}
func TestUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyUnauthorizedPublisherJWT(hub))
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestNoIRI(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedPublisherJWT(hub))
	req.Form = url.Values{}
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"iri\" parameter\n", w.Body.String())
}

func TestNoData(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("iri", "http://example.com/books/1")

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedPublisherJWT(hub))
	req.Form = form
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"data\" parameter\n", w.Body.String())
}

func TestOk(t *testing.T) {
	hub := createDummy()

	go func() {
		for {
			select {
			case content := <-hub.resources:
				assert.Equal(t, "http://example.com/books/1", content.IRI)
				assert.Equal(t, "data: Hello!\n", content.Data)
				assert.True(t, content.Targets["foo"])
				assert.True(t, content.Targets["bar"])
				return
			}
		}
	}()

	form := url.Values{}
	form.Add("iri", "http://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("target[]", "foo")
	form.Add("target[]", "bar")

	req := httptest.NewRequest("GET", "http://example.com/publish", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedPublisherJWT(hub))
	req.Form = form
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func createDummyAuthorizedPublisherJWT(h *Hub) string {
	token := jwt.New(jwt.SigningMethodHS256)

	expiresAt := time.Now().Add(time.Minute * 1).Unix()
	token.Claims = &jwt.StandardClaims{ExpiresAt: expiresAt}
	tokenString, _ := token.SignedString(h.publisherJWTKey)

	return tokenString
}

func createDummyUnauthorizedPublisherJWT(h *Hub) string {
	token := jwt.New(jwt.SigningMethodHS256)

	expiresAt := time.Now().Add(time.Minute * 1).Unix()
	token.Claims = &jwt.StandardClaims{ExpiresAt: expiresAt}
	tokenString, _ := token.SignedString([]byte("unauthorized"))

	return tokenString
}
