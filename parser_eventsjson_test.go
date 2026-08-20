package mercure

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The "url" member reaches parseMatchers in the shape the query component
// would have produced, so that one matcher implementation serves both forms.
func TestParseEventsJSONURL(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want url.Values
	}{
		{
			"an array is shorthand for the default matcher",
			`{"url": ["https://example.com/books/1"], "events": {}}`,
			url.Values{"match": {"https://example.com/books/1"}, "events": {""}},
		},
		{
			"an object names the matcher parameter",
			`{"url": {"match_urlpattern": ["https://example.com/books/:id"]}, "events": {}}`,
			url.Values{"match_urlpattern": {"https://example.com/books/:id"}, "events": {""}},
		},
		{
			"a scalar topic is one topic",
			`{"url": {"match": "https://example.com/books/1"}, "events": {}}`,
			url.Values{"match": {"https://example.com/books/1"}, "events": {""}},
		},
		{
			"several matchers together",
			`{"url": {"match": ["a"], "match_urlpattern": ["b", "c"]}, "events": {}}`,
			url.Values{"match": {"a"}, "match_urlpattern": {"b", "c"}, "events": {""}},
		},
		{
			"a null topic list carries no topic",
			`{"url": {"match": null}, "events": {}}`,
			url.Values{"match": nil, "events": {""}},
		},
		{
			"an absent url member carries none",
			`{"events": {}}`,
			url.Values{"events": {""}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			values, err := parseEventsJSON([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.want, values)
		})
	}
}

// Member names arrive unchanged: parseMatchers owns which are meaningful, so
// the two request forms cannot drift apart.
func TestParseEventsJSONPassesNamesThrough(t *testing.T) {
	t.Parallel()

	values, err := parseEventsJSON([]byte(`{"url": {"nonsense": ["a"]}, "events": {}}`))
	require.NoError(t, err)
	assert.Equal(t, url.Values{"nonsense": {"a"}, "events": {""}}, values)
}

// The "state" member asks for a representation, which a hub has none of.
func TestParseEventsJSONIgnoresState(t *testing.T) {
	t.Parallel()

	values, err := parseEventsJSON([]byte(`{"state": {"accept": "text/html"}, "url": ["a"], "events": {}}`))
	require.NoError(t, err)
	assert.Equal(t, url.Values{"match": {"a"}, "events": {""}}, values)
}

func TestParseEventsJSONErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
	}{
		{"not JSON at all", `nonsense`},
		{"an array, not an object", `["https://example.com/books/1"]`},
		{"url is a number", `{"url": 42, "events": {}}`},
		{"a topic is a number", `{"url": {"match": 42}, "events": {}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseEventsJSON([]byte(tc.body))
			require.Error(t, err)
		})
	}
}

// A request expresses its interest in a stream of notifications with the
// "events" member; whether it is there is all the hub reads from it.
func TestParseEventsJSONEventsMember(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"an events member asks for notifications", `{"url": ["a"], "events": {}}`, []string{""}},
		{"no events member", `{"url": ["a"]}`, nil},
		{"a null events member", `{"url": ["a"], "events": null}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			values, err := parseEventsJSON([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.want, values["events"])
		})
	}
}

// The cursor to resume from travels in "last_event_id", as it does in a query
// component. An empty cursor is still a cursor: the protocol answers with
// Mercure-Last-Event-Id whenever a subscription supplied one at all.
func TestParseEventsJSONLastEventID(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"a cursor", `{"url": ["a"], "events": {}, "last_event_id": "b"}`, []string{"b"}},
		{"an empty cursor", `{"url": ["a"], "events": {}, "last_event_id": ""}`, []string{""}},
		{"no cursor", `{"url": ["a"], "events": {}}`, nil},
		{"a null cursor", `{"url": ["a"], "events": {}, "last_event_id": null}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			values, err := parseEventsJSON([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.want, values["last_event_id"])
		})
	}
}

// A request the hub reads but cannot act on is unprocessable: this one asks
// for no events, the single notification of the specification's section 8.
func TestEventsQuerySubscribeJSONWithoutEvents(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithEventsQuery())

	req := httptest.NewRequest(methodQuery, defaultHubURL,
		strings.NewReader(`{"url": ["https://example.com/books/1"]}`))
	req.Header.Set("Content-Type", eventsJSONMediaType)

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// A hub not serving Events Query reads none of its requests: the media type
// is then one like any other the hub cannot parse, whatever parsers are
// registered.
func TestQuerySubscribeEventsJSONRejectedWhenDisabled(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)

	req := httptest.NewRequest(methodQuery, defaultHubURL,
		strings.NewReader(`{"url": ["https://example.com/books/1"], "events": {}}`))
	req.Header.Set("Content-Type", eventsJSONMediaType)

	w := httptest.NewRecorder()
	hub.SubscribeHandler(w, req)

	resp := w.Result()

	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
}
