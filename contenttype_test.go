package mercure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateValidateContentType(t *testing.T) {
	t.Parallel()

	base := Update{Topics: []string{"https://example.com/books/1"}}

	valid := base
	valid.ContentType = "application/ld+json; charset=utf-8"
	require.NoError(t, valid.Validate())

	for _, ct := range []string{"not a media type", "text/plain\r\nX-Injected: 1", "/missing-type"} {
		invalid := base
		invalid.ContentType = ct
		require.ErrorIs(t, invalid.Validate(), ErrInvalidMediaType, ct)
	}
}

// History entries persisted before the ContentType field existed must still
// decode, and the field must survive a marshal/unmarshal round trip.
func TestUpdateContentTypeJSONRoundTrip(t *testing.T) {
	t.Parallel()

	u := &Update{
		Topics:      []string{"https://example.com/books/1"},
		ContentType: "application/ld+json",
		Event:       Event{Data: "d", ID: "i"},
	}

	serialized, err := json.Marshal(u)
	require.NoError(t, err)

	var decoded Update

	require.NoError(t, json.Unmarshal(serialized, &decoded))
	assert.Equal(t, *u, decoded)

	var legacy Update

	require.NoError(t, json.Unmarshal([]byte(`{"Topics":["https://example.com/books/1"],"Data":"d","ID":"i"}`), &legacy))
	assert.Empty(t, legacy.ContentType)
}

func TestPublishHandlerContentType(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	// The subscriber is registered before the publication, so by the time
	// PublishHandler returns the update sits in its buffered channel.
	s := NewLocalSubscriber("", hub.logger, hub.topicMatcherStore)
	s.SetMatchers([]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/books/1"}}, nil)
	require.NoError(t, hub.transport.AddSubscriber(t.Context(), s))

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", "Hello World")
	form.Add("content_type", "application/ld+json")

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

	select {
	case dispatched := <-s.Receive():
		assert.Equal(t, "application/ld+json", dispatched.ContentType)
	case <-time.After(5 * time.Second):
		t.Fatal("update not received")
	}
}

func TestPublishHandlerInvalidContentType(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	form := url.Values{}
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", "Hello World")
	form.Add("content_type", "not a media type")

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
