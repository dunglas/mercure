package mercure

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionsHandlerAccessDenied(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, subscriptionsURL, nil)
	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, subscriptionsURL, nil)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo"})})

	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/bar", nil)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo"})})

	w = httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	require.NoError(t, res.Body.Close())
}

func TestSubscriptionHandlerAccessDenied(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/bar/baz", nil)
	w := httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/bar/baz", nil)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo"})})

	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	require.NoError(t, res.Body.Close())
}

// TestSubscriptionsHandlerAuthorizesAgainstPath verifies the subscription API
// authorizes against the request path only: a query string (here last_event_id)
// must not break an Exact subscribe grant that covers the path.
func TestSubscriptionsHandlerAuthorizesAgainstPath(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, subscriptionsURL+"?last_event_id=foo", nil)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestSubscriptionHandlersETag(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	req.Header.Add("If-None-Match", `"`+EarliestLastEventID+`"`)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusNotModified, res.StatusCode)
	require.NoError(t, res.Body.Close())

	req = httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath+"/foo/bar", nil)
	req.Header.Add("If-None-Match", `"`+EarliestLastEventID+`"`)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions/foo/bar"})})

	w = httptest.NewRecorder()
	hub.SubscriptionHandler(w, req)
	res = w.Result()
	assert.Equal(t, http.StatusNotModified, res.StatusCode)
	require.NoError(t, res.Body.Close())
}

func TestSubscriptionsHandler(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	tss := &TopicSelectorStore{}
	logger := slog.Default()
	ctx := t.Context()

	s1 := NewLocalSubscriber("", logger, tss)
	s1.setMatchers(stringsToExactMatchers([]string{"https://example.com/foo"}), nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", logger, tss)
	s2.setMatchers(stringsToExactMatchers([]string{"https://example.com/bar"}), nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, s2))

	req := httptest.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{"/.well-known/mercure/subscriptions"})})

	w := httptest.NewRecorder()
	hub.SubscriptionsHandler(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var subscriptions subscriptionCollection
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &subscriptions))

	assert.Equal(t, subscriptionsURL, subscriptions.ID)
	assert.Equal(t, "subscriptions", subscriptions.Type)

	last_event_id, subscribers, _ := hub.transport.(TransportSubscribers).GetSubscribers(t.Context())

	assert.Equal(t, last_event_id, subscriptions.LastEventID)
	require.NotEmpty(t, subscribers)

	for _, s := range subscribers {
		currentSubs := s.getSubscriptions(subscriptionFilter{}, true)
		require.NotEmpty(t, currentSubs)

		for _, sub := range currentSubs {
			assert.Contains(t, subscriptions.Subscriptions, sub)
		}
	}
}

// TestSubscriptionPayloadFromMatchingClaim verifies the spec rule: the payload
// attached to a subscription is the payload of the FIRST JWT subscribe-claim
// that matches the subscription's own matcher — not the claim at the same
// positional index.
func TestSubscriptionPayloadFromMatchingClaim(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	logger := slog.Default()

	sub := NewLocalSubscriber("", logger, hub.topicSelectorStore)
	matchers, err := hub.parseMatchers(url.Values{
		"match": {"https://example.com/foo", "https://example.com/bar"},
	}, false)
	require.NoError(t, err)

	sub.Claims = detailClaims(t, hub.topicSelectorStore,
		// Non-matching detail first — must not be picked.
		subscribeDetail(map[string]any{"tag": "x"}, TopicMatcher{Type: MatcherTypeExact, Pattern: "https://other.example.com/x"}),
		// This URLPattern detail covers /foo AND /bar → gets picked as "first matching".
		subscribeDetail(map[string]any{"tag": "urlpattern"}, TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "https://example.com/:id"}),
		// Exact detail for /bar — would only win if iteration reached it first.
		subscribeDetail(map[string]any{"tag": "exact-bar"}, TopicMatcher{Type: MatcherTypeExact, Pattern: "https://example.com/bar"}),
	)

	sub.setMatchers(matchers, nil)

	subs := sub.getSubscriptions(subscriptionFilter{}, true)
	require.Len(t, subs, 2)

	for _, s := range subs {
		p, ok := s.Payload.(map[string]any)
		require.True(t, ok, "payload must come from a matching detail")
		assert.Equal(t, "urlpattern", p["tag"], "first MATCHING detail wins, not the first detail by index")
	}
}

// TestSubscriptionHandlerMatchRoute exercises the
// /subscriptions/{match_type}/{match}/{subscriber} URL shape.
func TestSubscriptionHandlerMatchRoute(t *testing.T) {
	t.Parallel()

	hub := createDummy(t)
	ctx := t.Context()
	logger := slog.Default()

	sub := NewLocalSubscriber("", logger, hub.topicSelectorStore)
	matchers, err := hub.parseMatchers(url.Values{
		"match_urlpattern": {"https://example.com/:id"},
	}, false)
	require.NoError(t, err)
	sub.setMatchers(matchers, nil)
	require.NoError(t, hub.transport.AddSubscriber(ctx, sub))

	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)
	router.HandleFunc(subscriptionMatchURL, hub.SubscriptionHandler)

	// Use the escaped matcher directly from the subscriber to avoid encoding drift.
	authURL := "/.well-known/mercure/subscriptions/" + sub.EscapedMatchers[0] + "/" + sub.EscapedID

	req := httptest.NewRequest(http.MethodGet, authURL, nil)
	req.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyAuthorizedJWT(roleSubscriber, []string{authURL})})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, res.Body.Close())

	var got subscription
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	assert.Equal(t, "https://example.com/:id", got.Match)
	assert.Equal(t, "urlpattern", got.MatchType)
	assert.Empty(t, got.Topic, "modern subscriptions must not emit the deprecated `topic` field")
}

// TestEscapeSubscriptionSegmentRoundTrip verifies the segment encoder
// produces only RFC 3986 unreserved characters and %XX sequences (a
// requirement for v8 URI-template subscription auth) AND that the
// resulting slug round-trips through url.PathUnescape — the decoder used
// by filterFromVars and isKnownMatchType. Literal '+' from a hand-built
// client URL must also decode to literal '+'.
func TestEscapeSubscriptionSegmentRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in       string
		wantBack string // what PathUnescape recovers
	}{
		{"https://example.com/foo", "https://example.com/foo"},
		{"foo+bar", "foo+bar"}, // server-encoded literal '+'
		{"foo bar", "foo bar"}, // space round-trips through %20
		{"a:b", "a:b"},         // ':' percent-encoded by encoder
		{"x?y&z=1", "x?y&z=1"}, // query-style chars
		{"https://example.com/{id}", "https://example.com/{id}"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			escaped := escapeSubscriptionSegment(tc.in)
			// Output must contain only unreserved + %XX so URI Template
			// `{var}` matching keeps working.
			assert.NotContains(t, escaped, "+", "encoder must use %20, not '+', for spaces")

			got, err := url.PathUnescape(escaped)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBack, got)
		})
	}

	// Literal '+' in a client-constructed URL decodes as literal '+',
	// not as a space.
	got, err := url.PathUnescape("foo+bar")
	require.NoError(t, err)
	assert.Equal(t, "foo+bar", got)
}
