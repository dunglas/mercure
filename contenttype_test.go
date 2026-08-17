package mercure

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
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

// newMultipartPublishRequest builds a multipart/form-data publication: every
// field value plus a data part carrying raw bytes and, optionally, an
// explicit Content-Type header.
func newMultipartPublishRequest(t *testing.T, fields url.Values, data []byte, dataContentType string) *http.Request {
	t.Helper()

	var body bytes.Buffer

	mw := multipart.NewWriter(&body)

	for name, values := range fields {
		for _, v := range values {
			require.NoError(t, mw.WriteField(name, v))
		}
	}

	header := textproto.MIMEHeader{"Content-Disposition": {`form-data; name="data"`}}
	if dataContentType != "" {
		header.Set("Content-Type", dataContentType)
	}

	pw, err := mw.CreatePart(header)
	require.NoError(t, err)

	_, err = pw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, &body)
	req.Header.Add("Content-Type", mw.FormDataContentType())
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	return req
}

func TestPublishHandlerMultipart(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithEventsQuery())

	// The subscriber is registered before the publication, so by the time
	// PublishHandler returns the update sits in its buffered channel.
	s := NewLocalSubscriber("", hub.logger, hub.topicMatcherStore)
	s.SetMatchers([]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/books/1"}}, nil)
	require.NoError(t, hub.transport.AddSubscriber(t.Context(), s))

	binaryData := []byte{0x89, 'P', 'N', 'G', 0xff, 0x00, 0xfe}
	form := url.Values{"topic": {"https://example.com/books/1"}}

	req := newMultipartPublishRequest(t, form, binaryData, "image/png")

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case dispatched := <-s.Receive():
		assert.Equal(t, string(binaryData), dispatched.Data)
		assert.Equal(t, "image/png", dispatched.ContentType)
		assert.True(t, dispatched.Binary)
	case <-time.After(5 * time.Second):
		t.Fatal("update not received")
	}
}

func TestPublishHandlerMultipartInvalidContentType(t *testing.T) {
	t.Parallel()

	hub := createDummy(t, WithEventsQuery())

	form := url.Values{"topic": {"https://example.com/books/1"}}
	req := newMultipartPublishRequest(t, form, []byte("Hello World"), "not a media type")

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// Multipart publication belongs to the experimental events query surface:
// without the option the hub must not silently parse an empty form.
func TestPublishHandlerMultipartDisabled(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	form := url.Values{"topic": {"https://example.com/books/1"}}
	req := newMultipartPublishRequest(t, form, []byte("Hello World"), "text/plain")

	w := httptest.NewRecorder()
	hub.PublishHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
}
