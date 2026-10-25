package mercure

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	wurl "github.com/nlnwa/whatwg-url/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fuzz targets live in this file and depend only on non-test code: OSS-Fuzz compiles this file alone.

// newFuzzPublishHub returns a hub and an Authorization header allowed to publish to any topic.
func newFuzzPublishHub(f *testing.F) (*Hub, string) {
	f.Helper()

	const issuer, audience, key = "https://example.com", "https://example.com/.well-known/mercure", "publisher"

	hub, err := NewHub(context.Background(),
		WithIssuers([]Issuer{{Identifier: issuer, Publisher: Static{Key: []byte(key), Algorithm: "HS256"}}}),
		WithResourceIdentifier(audience),
	)
	require.NoError(f, err)

	token := jwt.New(jwt.SigningMethodHS256)
	token.Header["typ"] = atJWTType
	token.Claims = &claims{
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		AuthorizationDetails: []authorizationDetail{{
			Type:    authorizationDetailTypeMercure,
			Actions: []mercureAction{actionPublish},
			Topics:  []detailTopic{{TopicMatcher{Type: MatcherTypeExact, Pattern: "*"}}},
		}},
	}

	signed, err := token.SignedString([]byte(key))
	require.NoError(f, err)

	return hub, bearerPrefix + signed
}

func FuzzPublish(f *testing.F) {
	hub, authorizationHeader := newFuzzPublishHub(f)

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

func FuzzReservedNamespaceValidation(f *testing.F) {
	base, err := wurl.Parse(urlPatternFallbackBase)
	require.NoError(f, err)

	for _, topic := range []string{
		"https://example.com/.well-known/mercure",
		"/.well-known/mercure/subscriptions/foo",
		"mercure", "merc%75re", "merc\nure", "merc\tur\re",
		"", "?q", "#f", "*", "//host/mercure", `https://example.com\.well-known\mercure`,
		"https:", "..", "/.well-known/./mercure", "%2e%77ell-known/mercure",
		"HTTPS://EXAMPLE.COM/.WELL-KNOWN/MERCURE", "bar",
	} {
		f.Add(topic)
	}

	f.Fuzz(func(t *testing.T, topic string) {
		resolved, err := base.Parse(topic)
		if err != nil || !pathAddressesReservedNamespace(resolved) {
			return
		}

		update := &Update{Topics: []string{topic}}
		if !validProtocolString(topic) {
			require.ErrorIs(t, update.Validate(urlPatternFallbackBase), ErrInvalidTopic)
		} else {
			require.ErrorIs(t, update.Validate(urlPatternFallbackBase), ErrReservedTopic)
		}
	})
}

func FuzzReservedTopicRejectedWhenPatternMatches(f *testing.F) {
	patterns := []TopicMatcher{
		{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure/subscriptions/exact/*"},
		{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure/subscriptions/*"},
		{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure/*"},
		// The hub URL itself is reserved, and a reference carrying only a scheme
		// reaches it without reaching any pattern nested under it.
		{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure"},
		{Type: MatcherTypeExact, Pattern: "/.well-known/mercure"},
		{Type: MatcherTypeExact, Pattern: "/.well-known/mercure/subscriptions/exact/x/y"},
	}
	if deprecatedMatcherTypeCompiled() {
		patterns = append(patterns, TopicMatcher{Type: deprecatedMatcherTypeName, Pattern: "/.well-known/mercure/subscriptions/{+topic}"})
	}

	stores := make(map[string]*TopicMatcherStore)

	for _, base := range []string{
		"", // the synthetic fallback
		"https://hub.example/.well-known/mercure",
		"http://hub.example/.well-known/mercure",
		"ws://hub.example/.well-known/mercure",
		"https://hub.example/nested/.well-known/mercure",
	} {
		tms, err := NewTopicMatcherStore(0)
		require.NoError(f, err)
		require.NoError(f, tms.setBaseURL(base))

		stores[base] = tms
	}

	for _, topic := range []string{
		"https:mercure/subscriptions/exact/x/y",
		"/.well-known/mercure/subscriptions/../../https://example.com",
		"/.well-known/mercure/subscriptions/exact/x/y",
		"mercure/subscriptions/exact/x/y",
		"https://hub.example/.well-known/mercure/x",
		"http:mercure/x", "ws:mercure/x", "//hub.example/.well-known/mercure/x",
		"%2e%77ell-known/mercure/x", "?q", "#f", "", "*", "bar",
		"https:", "http:", "ws:", "HTTPS:", " https:", " https:mercure/", "https:?q", "https:#f",
	} {
		f.Add(topic)
	}

	f.Fuzz(func(t *testing.T, topic string) {
		// Validate rejects these first, on other grounds.
		if !validProtocolString(topic) {
			return
		}

		for base, tms := range stores {
			for _, pattern := range patterns {
				if !tms.matches([]string{topic}, pattern) {
					continue
				}

				update := &Update{Topics: []string{topic}}
				require.ErrorIs(t, update.Validate(tms.base()), ErrReservedTopic,
					"topic %q matches reserved pattern %q (type %q, base %q)", topic, pattern.Pattern, pattern.Type, base)
			}
		}
	})
}
