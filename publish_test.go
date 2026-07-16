package mercure

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublish(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hub := createDummy(t)

		topics := []string{"https://example.com/books/1"}
		s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
		s.setMatchers(stringsToExactMatchers(topics), stringsToExactMatchers(topics))

		require.NoError(t, hub.transport.AddSubscriber(t.Context(), s))

		go func() {
			u, ok := <-s.Receive()

			assert.True(t, ok)
			assert.NotNil(t, u)
			assert.Equal(t, "id", u.ID)
			assert.Equal(t, s.SubscribedMatchers[0].Pattern, u.Topic)
			assert.Equal(t, "Hello!", u.Data)
			assert.True(t, u.Private)
		}()

		require.NoError(t, hub.Publish(t.Context(), &Update{
			Event: Event{
				ID:   "id",
				Data: "Hello!",
			},
			Topic:   s.SubscribedMatchers[0].Pattern,
			Private: true,
		}))

		synctest.Wait()
	})
}

func TestPublishHandlerNoAuthorizationHeader(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

func TestPublishHandlerUnauthorizedJWT(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

func TestPublishHandlerInvalidAlgJWT(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

func TestPublishHandlerBadContentType(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

func TestPublishHandlerNoTopic(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

func TestPublishHandlerInvalidRetry(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

func TestPublishHandlerNotAuthorizedTopicSelector(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPublishHandlerEmptyTopicSelector(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestPublishHandlerLegacyAuthorization(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithProtocolVersionCompatibility(7))

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

func TestPublishHandlerOK(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		hub := createDummy(t)

		topics := []string{"https://example.com/books/1"}
		s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
		s.setMatchers(stringsToExactMatchers(topics), stringsToExactMatchers(topics))

		require.NoError(t, hub.transport.AddSubscriber(t.Context(), s))

		go func() {
			u, ok := <-s.Receive()
			assert.True(t, ok)
			assert.NotNil(t, u)
			assert.Equal(t, "id", u.ID)
			assert.Equal(t, s.SubscribedMatchers[0].Pattern, u.Topic)
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
		req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, topics))

		w := httptest.NewRecorder()
		hub.PublishHandler(w, req)

		resp := w.Result()

		t.Cleanup(func() {
			assert.NoError(t, resp.Body.Close())
		})

		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Equal(t, "id", string(body))

		synctest.Wait()
	})
}

func TestPublishHandlerNoData(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

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

func TestPublishHandlerGenerateUUID(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		h := createDummy(t)

		s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
		s.setMatchers(stringsToExactMatchers([]string{"https://example.com/books/1"}), nil)

		require.NoError(t, h.transport.AddSubscriber(t.Context(), s))

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

func TestPublishHandlerWithErrorInTransport(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	require.NoError(t, hub.transport.Close(t.Context()))

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

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "Internal Server Error\n", string(body))
}

func FuzzPublish(f *testing.F) {
	hub := createDummy(f)
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

func TestUpdateValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		update Update
		want   error
	}{
		{"valid", Update{Event: Event{ID: "id", Type: "type"}, Topic: "https://example.com/books/1"}, nil},
		// The empty topic resolves to the hub URL itself, which is reserved.
		{"empty", Update{}, ErrReservedTopic},
		{"reserved topic", Update{Topic: "https://example.com/.well-known/mercure/subscriptions/foo"}, ErrReservedTopic},
		{"reserved topic relative", Update{Topic: "mercure/subscriptions/foo"}, ErrReservedTopic},
		{"reserved topic absolute path", Update{Topic: "/.well-known/mercure/subscriptions/foo"}, ErrReservedTopic},
		{"reserved topic exact", Update{Topic: "https://example.com/.well-known/mercure"}, ErrReservedTopic},
		{"reserved topic percent-encoded", Update{Topic: "https://example.com/.well-known/%6Dercure/subscriptions/foo"}, ErrReservedTopic},
		{"reserved topic backslashes", Update{Topic: `https://example.com\.well-known\mercure\subscriptions\foo`}, ErrReservedTopic},
		{"reserved wildcard", Update{Topic: "*"}, ErrReservedWildcard},
		{"non-reserved mid-path namespace", Update{Topic: "https://example.com/foo/.well-known/mercure/bar"}, nil},
		{"non-reserved sibling path", Update{Topic: "https://example.com/.well-known/mercure-dashboard"}, nil},
		{"non-reserved opaque topic", Update{Topic: "urn:example:mercure"}, nil},
		{"id starts with #", Update{Topic: "https://example.com/books/1", Event: Event{ID: "#42"}}, ErrInvalidEventID},
		{"id earliest", Update{Topic: "https://example.com/books/1", Event: Event{ID: EarliestLastEventID}}, ErrInvalidEventID},
		{"topic NUL", Update{Topic: "https://example.com/foo\x00bar"}, ErrInvalidTopic},
		{"topic C0", Update{Topic: "https://example.com/foo\nbar"}, ErrInvalidTopic},
		{"topic invalid UTF-8", Update{Topic: "https://example.com/\xff"}, ErrInvalidTopic},
		{"id LF", Update{Topic: "https://example.com/books/1", Event: Event{ID: "foo\nevent: injected"}}, ErrInvalidEventID},
		{"id CR", Update{Topic: "https://example.com/books/1", Event: Event{ID: "foo\rinjected"}}, ErrInvalidEventID},
		{"id NUL", Update{Topic: "https://example.com/books/1", Event: Event{ID: "foo\x00bar"}}, ErrInvalidEventID},
		{"type LF", Update{Topic: "https://example.com/books/1", Event: Event{Type: "foo\nid: injected"}}, ErrInvalidEventType},
		{"type CR", Update{Topic: "https://example.com/books/1", Event: Event{Type: "foo\rinjected"}}, ErrInvalidEventType},
		{"type NUL", Update{Topic: "https://example.com/books/1", Event: Event{Type: "foo\x00bar"}}, ErrInvalidEventType},
		{"type reserved mercure", Update{Topic: "https://example.com/books/1", Event: Event{Type: reservedEventType}}, ErrReservedEventType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.update.Validate()
			if tc.want == nil {
				assert.NoError(t, err)

				return
			}

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestPublishHandlerReservedTopicNamespace(t *testing.T) {
	t.Parallel()

	for _, topic := range []string{
		"/.well-known/mercure/subscriptions/foo",
		"https://example.com/.well-known/mercure/subscriptions/foo",
		"mercure/subscriptions/foo", // relative, resolves into the namespace
		"https://example.com/.well-known/%6Dercure/subscriptions/foo",
	} {
		t.Run(topic, func(t *testing.T) {
			t.Parallel()

			hub := createDummy(t)

			form := url.Values{}
			form.Add("topic", topic)
			form.Add("data", "Hello!")

			req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

			w := httptest.NewRecorder()
			hub.PublishHandler(w, req)

			resp := w.Result()

			t.Cleanup(func() {
				assert.NoError(t, resp.Body.Close())
			})

			// The reserved namespace is off-limits to every publisher
			// regardless of grants, so it is a request-validation failure
			// (400 invalid_request), not an authorization failure.
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestPublishHandlerRejectsSSEControlChars(t *testing.T) {
	t.Parallel()

	cases := []struct {
		field string
		value string
	}{
		{"id", "foo\nevent: injected"},
		{"id", "foo\revent: injected"},
		{"id", "foo\x00bar"},
		{"type", "foo\nid: injected"},
		{"type", "foo\rdata: injected"},
		{"type", "foo\x00bar"},
		// "mercure" is reserved for hub-generated events; a publisher using it
		// must be rejected (exercises the PublishHandler ErrReservedEventType arm).
		{"type", "mercure"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%q", tc.field, tc.value), func(t *testing.T) {
			t.Parallel()

			hub := createDummy(t)

			form := url.Values{}
			form.Add("topic", "https://example.com/books/1")
			form.Add(tc.field, tc.value)
			form.Add("data", "Hello!")

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
		})
	}
}

func TestPublishHandlerTooManyTopics(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	form := url.Values{}
	for i := 0; i <= maxPublishTopics; i++ {
		form.Add("topic", "https://example.com/books/1")
	}

	form.Add("data", "Hello!")

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
}

func TestPublishHandlerTooManyClaimMatchers(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	scope := make([]string, maxClaimMatchers+1)
	for i := range scope {
		scope[i] = "https://example.com/books/1"
	}

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", "Hello!")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, scope))

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	// Too many topics in a single authorization detail → invalid_token.
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
