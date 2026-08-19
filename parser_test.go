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
