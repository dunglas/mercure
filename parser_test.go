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
