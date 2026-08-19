package mercure

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A subscription refusing every media type the hub answers with leaves
// nothing to send it.
func TestEventsQuerySubscribeRefusedResponseMediaType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	req := eventsQueryRequest(t.Context(), "match=https://example.com/books/1&events=")
	req.Header.Set("Accept", eventStreamContentType+";q=0")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
}

// A subscription that named no media type it will read is not refusing any,
// and one that named several is served whichever it weighted highest. Only a
// subscription refusing all of them leaves nothing to send.
func TestNegotiate(t *testing.T) {
	t.Parallel()

	const ndjson = "application/x-ndjson"

	offered := parseMediaTypes([]string{eventStreamContentType, ndjson})

	for name, tc := range map[string]struct {
		accept []string
		want   string
	}{
		"absent":     {nil, eventStreamContentType},
		"wildcard":   {[]string{"*/*"}, eventStreamContentType},
		"named":      {[]string{ndjson}, ndjson},
		"weighted":   {[]string{eventStreamContentType + ";q=0.3, " + ndjson + ";q=0.8"}, ndjson},
		"split":      {[]string{eventStreamContentType + ";q=0.3", ndjson + ";q=0.8"}, ndjson},
		"unnamed":    {[]string{"application/json"}, ""},
		"refused":    {[]string{eventStreamContentType + ";q=0", ndjson + ";q=0"}, ""},
		"unreadable": {[]string{"text/"}, eventStreamContentType},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
			for _, accept := range tc.accept {
				req.Header.Add("Accept", accept)
			}

			assert.Equal(t, tc.want, negotiate(req, offered))
		})
	}
}

// A subscription refusing the only media type an ordinary subscription is
// served in leaves nothing to send it, whether it asked with GET or QUERY.
func TestSubscribeRefusedResponseMediaType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(http.MethodGet,
		defaultHubURL+"?match=https://example.com/books/1", nil)
	req.Header.Set("Accept", eventStreamContentType+";q=0")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
}

func TestQuerySubscribeRefusedResponseMediaType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(methodQuery, defaultHubURL,
		strings.NewReader("match=https://example.com/books/1"))
	req.Header.Set("Content-Type", urlEncodedMediaType)
	req.Header.Set("Accept", eventStreamContentType+";q=0")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
}

// What the hub would send is settled only once it knows the request can be
// served at all, so a body it cannot read is refused before the response is
// negotiated.
func TestQuerySubscribeUnsupportedMediaTypeBeforeRefusedResponseMediaType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(methodQuery, defaultHubURL,
		strings.NewReader(`{"url": ["https://example.com/books/1"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", eventStreamContentType+";q=0")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
}

// The same order inside the Events Query dispatch: a subscription asking for
// no events is unprocessable however it would have been answered.
func TestEventsQuerySubscribeWithoutEventsBeforeRefusedResponseMediaType(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	req := eventsQueryRequest(t.Context(), "match=https://example.com/books/1")
	req.Header.Set("Accept", eventStreamContentType+";q=0")

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

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
