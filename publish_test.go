package mercure

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPublishNoAuthorizationHeader(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishUnauthorizedJWT(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyUnauthorizedJWT())

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishInvalidAlgJWT(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyNoneSignedJWT())

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusText(http.StatusUnauthorized)+"\n", w.Body.String())
}

func TestPublishBadContentType(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))
	req.Header.Add("Content-Type", "text/plain; boundary=")

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestPublishNoTopic(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, nil)
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, `Missing "topic" parameter
`, w.Body.String())
}

func TestPublishInvalidRetry(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", "foo")
	form.Add("retry", "invalid")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, `Invalid "retry" parameter
`, w.Body.String())
}

func TestPublishNotAuthorizedTopicSelector(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", "foo")
	form.Add("private", "on")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"foo"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPublishEmptyTopicSelector(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPublishLegacyAuthorization(t *testing.T) {
	t.Parallel()

	hub := createDummy(WithProtocolVersionCompatibility(7))

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPublishOK(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	topics := []string{"https://example.com/books/1"}
	s := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.SetTopics(topics, topics)
	s.Claims = &claims{Mercure: mercureClaim{Subscribe: topics}}

	require.NoError(t, hub.transport.AddSubscriber(s))

	synctest.Test(t, func(t *testing.T) {
		go func() {
			u, ok := <-s.Receive()
			assert.True(t, ok)
			assert.NotNil(t, u)
			assert.Equal(t, "id", u.ID)
			assert.Equal(t, s.SubscribedTopics, u.Topics)
			assert.Equal(t, "Hello!", u.Data)
			assert.True(t, u.Private)
		}()

		form := url.Values{}
		form.Add("id", "id")
		form.Add("topic", "https://example.com/books/1")
		form.Add("data", "Hello!")
		form.Add("private", "on")

		req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, s.SubscribedTopics))

		w := httptest.NewRecorder()
		hub.PublishHandler(w, req)

		resp := w.Result()

		t.Cleanup(func() {
			assert.NoError(t, resp.Body.Close())
		})

		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "id", string(body))

		synctest.Wait()
	})
}

func TestPublishNoData(t *testing.T) {
	t.Parallel()

	hub := createDummy()

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPublishGenerateUUID(t *testing.T) {
	t.Parallel()

	h := createDummy()

	s := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.SetTopics([]string{"https://example.com/books/1"}, s.SubscribedTopics)

	require.NoError(t, h.transport.AddSubscriber(s))

	synctest.Test(t, func(t *testing.T) {
		go func() {
			u := <-s.Receive()
			assert.NotNil(t, u)

			_, err := uuid.FromString(strings.TrimPrefix(u.ID, "urn:uuid:"))
			assert.NoError(t, err)
		}()

		form := url.Values{}
		form.Add("topic", "https://example.com/books/1")
		form.Add("data", "Hello!")

		req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

		w := httptest.NewRecorder()
		h.PublishHandler(w, req)

		resp := w.Result()

		t.Cleanup(func() {
			assert.NoError(t, resp.Body.Close())
		})

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)

		_, err := uuid.FromString(strings.TrimPrefix(body, "urn:uuid:"))
		require.NoError(t, err)

		synctest.Wait()
	})
}

func TestPublishWithErrorInTransport(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	hub := createDummy()
	require.NoError(t, hub.transport.Close())

	form := url.Values{}
	form.Add("id", "id")
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("private", "on")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"foo", "https://example.com/books/1"}))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "id", string(body))
}

func FuzzPublish(f *testing.F) {
	hub := createDummy()
	authorizationHeader := bearerPrefix + createDummyAuthorizedJWT(rolePublisher, []string{"*"})

	testCases := []struct {
		topic1, topic2, id, data, private, retry, typ string
	}{
		{"https://localhost/foo/bar", "baz", "", "", "", "", ""},
		{"https://localhost/foo/baz", "bat", "id", "data", "on", "22", "mytype"},
	}

	for _, tc := range testCases {
		f.Add(tc.topic1, tc.topic2, tc.id, tc.data, tc.private, tc.retry, tc.typ)
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

		t.Cleanup(func() {
			assert.NoError(t, resp.Body.Close())
		})

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusBadRequest {
			return
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		if id == "" {
			assert.NotEmpty(t, string(body))

			return
		}

		assert.Equal(t, id, string(body))
	})
}
