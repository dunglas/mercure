package hub

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoAuthorizationHeader(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", defaultHubURL, nil)
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", defaultHubURL, nil)
	req.Header.Add("Authorization", "Bearer "+createDummyUnauthorizedJWT())
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishInvalidAlgJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", defaultHubURL, nil)
	req.Header.Add("Authorization", "Bearer "+createDummyNoneSignedJWT())
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishBadContentType(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", defaultHubURL, nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{}))
	req.Header.Add("Content-Type", "text/plain; boundary=")
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPublishNoTopic(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest("POST", defaultHubURL, nil)
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{}))
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"topic\" parameter\n", w.Body.String())
}

func TestPublishNoData(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")

	req := httptest.NewRequest("POST", defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"data\" parameter\n", w.Body.String())
}

func TestPublishInvalidRetry(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "foo")
	form.Add("retry", "invalid")

	req := httptest.NewRequest("POST", defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Invalid \"retry\" parameter\n", w.Body.String())
}

func TestPublishNotAuthorizedTarget(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "foo")
	form.Add("target", "not-allowed")

	req := httptest.NewRequest("POST", defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{"foo"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPublishOK(t *testing.T) {
	hub := createDummy()

	pipe, err := hub.transport.CreatePipe("")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		u, err := pipe.Read(context.Background())
		assert.Nil(t, err)
		require.NotNil(t, u)
		assert.Equal(t, "id", u.ID)
		assert.Equal(t, []string{"http://example.com/books/1"}, u.Topics)
		assert.Equal(t, "Hello!", u.Data)
		assert.Equal(t, struct{}{}, u.Targets["foo"])
		assert.Equal(t, struct{}{}, u.Targets["bar"])
	}(&wg)

	form := url.Values{}
	form.Add("id", "id")
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("target", "foo")
	form.Add("target", "bar")

	req := httptest.NewRequest("POST", defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{"foo", "bar"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id", string(body))

	wg.Wait()
}

func TestPublishGenerateUUID(t *testing.T) {
	hub := createDummy()

	pipe, err := hub.transport.CreatePipe("")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		u, err := pipe.Read(context.Background())
		assert.Nil(t, err)
		require.NotNil(t, u)
		_, err = uuid.FromString(u.ID)
		assert.Nil(t, err)
	}(&wg)

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "Hello!")

	req := httptest.NewRequest("POST", defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyAuthorizedJWT(hub, publisherRole, []string{})})
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	body := string(bodyBytes)

	_, err = uuid.FromString(body)
	assert.Nil(t, err)
}

func TestPublishWithErrorInTransport(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	hub := createDummy()
	hub.transport.Close()

	form := url.Values{}
	form.Add("id", "id")
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("target", "foo")
	form.Add("target", "bar")

	req := httptest.NewRequest("POST", defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(hub, publisherRole, []string{"foo", "bar"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id", string(body))
}
