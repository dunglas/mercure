package mercure

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A QUERY body the hub cannot parse is a bad request. Only a body can be
// malformed this way: the query component of a GET is parsed leniently.
func TestQuerySubscribeMalformedBodyRejectedWith400(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader("match=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	// The status text, not the matcher error: a body that will not parse is
	// refused before any matcher is looked for.
	assert.Equal(t, http.StatusText(http.StatusBadRequest)+"\n", w.Body.String())
}

// A QUERY naming a media type the hub cannot read as a subscription is
// unsupported: its content is not read as a form (RFC 10008, Section 2.3).
func TestQuerySubscribeUnsupportedMediaTypeRejectedWith415(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(methodQuery, defaultHubURL,
		strings.NewReader(`{"url": ["https://example.com/books/1"]}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
}

// A QUERY naming no media type at all is incorrect by definition, so it is a
// bad request rather than an unsupported one (RFC 10008, Section 2.3).
func TestQuerySubscribeWithoutMediaTypeRejectedWith400(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(methodQuery, defaultHubURL,
		strings.NewReader("match=https://example.com/books/1"))

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// The same two rules apply inside the Events Query dispatch, which reads the
// media type to choose a parser.
func TestEventsQuerySubscribeUnsupportedMediaType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
}

func TestEventsQuerySubscribeWithoutMediaType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	req := httptest.NewRequest(methodQuery, defaultHubURL, strings.NewReader(`{}`))

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// A QUERY served as an Events Query carries its subscription in the body, so
// a matcher in the query component is not one.
func TestEventsQuerySubscribeIgnoresURLParameters(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	req := httptest.NewRequest(methodQuery,
		defaultHubURL+"?match=https://example.com/books/1", strings.NewReader("events="))
	req.Header.Set("Content-Type", urlEncodedMediaType)

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "missing \"match\" subscription parameter\n", w.Body.String())
}

// A QUERY the hub reads and understands but that asks for no events is
// unprocessable: a stream of notifications is the only mode it serves.
func TestEventsQuerySubscribeWithoutEvents(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	req := httptest.NewRequest(methodQuery, defaultHubURL,
		strings.NewReader("match=https://example.com/books/1"))
	req.Header.Set("Content-Type", urlEncodedMediaType)

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}
