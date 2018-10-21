package hub

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func TestNoAuthorizationHeader(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", "http://example.com/hub", nil)
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", "http://example.com/hub", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyUnauthorizedJWT())
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishInvalidAlgJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", "http://example.com/hub", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyNoneSignedJWT())
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishBadContentType(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", "http://example.com/hub", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true, []string{}))
	req.Header.Add("Content-Type", "text/plain; boundary=")
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPublishNoTopic(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", "http://example.com/hub", nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true, []string{}))
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"topic\" parameter\n", w.Body.String())
}

func TestPublishNoData(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")

	req := httptest.NewRequest("POST", "http://example.com/hub", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"data\" parameter\n", w.Body.String())
}

func TestPublishInvalidRetry(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "foo")
	form.Add("retry", "invalid")

	req := httptest.NewRequest("POST", "http://example.com/hub", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Invalid \"retry\" parameter\n", w.Body.String())
}

func TestPublishNotAuthorizedTarget(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "foo")
	form.Add("target", "not-allowed")

	req := httptest.NewRequest("POST", "http://example.com/hub", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true, []string{"foo"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPublishOK(t *testing.T) {
	hub := createDummy()

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		for {
			select {
			case u := <-hub.updates:
				assert.Equal(t, "id", u.ID)
				assert.Equal(t, []string{"http://example.com/books/1"}, u.Topics)
				assert.Equal(t, "Hello!", u.Data)
				assert.Equal(t, struct{}{}, u.Targets["foo"])
				assert.Equal(t, struct{}{}, u.Targets["bar"])
				return
			}
		}
	}(&wg)

	form := url.Values{}
	form.Add("id", "id")
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("target", "foo")
	form.Add("target", "bar")

	req := httptest.NewRequest("POST", "http://example.com/hub", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true, []string{"foo", "bar"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id", string(body))

	wg.Wait()
}

func TestPublishGenerateUUID(t *testing.T) {
	hub := createDummy()

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		for {
			select {
			case u := <-hub.updates:
				_, err := uuid.FromString(u.ID)
				assert.Nil(t, err)
				return
			}
		}
	}(&wg)

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "Hello!")

	req := httptest.NewRequest("POST", "http://example.com/hub", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, true, []string{})})
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, true, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	body := string(bodyBytes)

	_, err := uuid.FromString(body)
	assert.Nil(t, err)
}
