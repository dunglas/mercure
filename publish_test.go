package mercure

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPublishNoAuthorizationHeader(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishUnauthorizedJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyUnauthorizedJWT())
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishInvalidAlgJWT(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyNoneSignedJWT())
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishBadContentType(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))
	req.Header.Add("Content-Type", "text/plain; boundary=")
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPublishNoTopic(t *testing.T) {
	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Missing \"topic\" parameter\n", w.Body.String())
}

func TestPublishInvalidRetry(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "foo")
	form.Add("retry", "invalid")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "Invalid \"retry\" parameter\n", w.Body.String())
}

func TestPublishNotAuthorizedTopicSelector(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "foo")
	form.Add("private", "on")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"foo"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPublishEmptyTopicSelector(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPublishLegacyAuthorization(t *testing.T) {
	hub := createDummy(WithProtocolVersionCompatibility(7))

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPublishOK(t *testing.T) {
	hub := createDummy()

	topics := []string{"http://example.com/books/1"}
	s := NewSubscriber("", zap.NewNop())
	s.SetTopics(topics, topics)
	s.Claims = &claims{Mercure: mercureClaim{Subscribe: topics}}

	require.NoError(t, hub.transport.AddSubscriber(s))

	var wg sync.WaitGroup
	wg.Add(1)
	go func(w *sync.WaitGroup) {
		defer w.Done()
		u, ok := <-s.Receive()
		assert.True(t, ok)
		assert.NotNil(t, u)
		assert.Equal(t, "id", u.ID)
		assert.Equal(t, s.SubscribedTopics, u.Topics)
		assert.Equal(t, "Hello!", u.Data)
		assert.True(t, u.Private)
	}(&wg)

	form := url.Values{}
	form.Add("id", "id")
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("private", "on")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, s.SubscribedTopics))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id", string(body))

	wg.Wait()
}

func TestPublishNoData(t *testing.T) {
	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPublishGenerateUUID(t *testing.T) {
	h := createDummy()

	s := NewSubscriber("", zap.NewNop())
	s.SetTopics([]string{"http://example.com/books/1"}, s.SubscribedTopics)

	require.NoError(t, h.transport.AddSubscriber(s))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		u := <-s.Receive()
		assert.NotNil(t, u)

		_, err := uuid.FromString(strings.TrimPrefix(u.ID, "urn:uuid:"))
		assert.NoError(t, err)
	}()

	form := url.Values{}
	form.Add("topic", "http://example.com/books/1")
	form.Add("data", "Hello!")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	h.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	_, err := uuid.FromString(strings.TrimPrefix(body, "urn:uuid:"))
	require.NoError(t, err)

	wg.Wait()
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
	form.Add("private", "on")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"foo", "http://example.com/books/1"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id", string(body))
}

func FuzzPublish(f *testing.F) {
	hub := createDummy()
	authorizationHeader := bearerPrefix + createDummyAuthorizedJWT(rolePublisher, []string{"*"})

	testCases := [][]interface{}{
		{"https://localhost/foo/bar", "baz", "", "", "", "", ""},
		{"https://localhost/foo/baz", "bat", "id", "data", "on", "22", "mytype"},
	}

	for _, tc := range testCases {
		f.Add(tc...) //nolint:govet
	}

	f.Fuzz(func(t *testing.T, topic1, topic2, id, data, private, retry, typ string) {
		form := url.Values{}
		form.Add("topic", topic1)
		form.Add("topic", topic2)
		form.Add("id", id)
		form.Add("data", data)
		form.Add("private", private)
		form.Add("retry", retry)
		form.Add("type", typ)

		req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Authorization", authorizationHeader)

		w := httptest.NewRecorder()
		hub.PublishHandler(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusBadRequest {
			return
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		if id == "" {
			assert.NotEqual(t, "", string(body))

			return
		}

		assert.Equal(t, id, string(body))
	})
}
