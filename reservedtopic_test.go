package mercure

import (
	"testing"

	wurl "github.com/nlnwa/whatwg-url/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressesReservedNamespace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		topic    string
		reserved bool
	}{
		// Absolute URLs: the path is checked regardless of scheme and authority.
		{"https://example.com/.well-known/mercure", true},
		{"https://example.com/.well-known/mercure/subscriptions/foo", true},
		{"http://other.example/.well-known/mercure/x", true},
		{"https://example.com/.well-known/mercureXXX", false},
		{"https://example.com/.well-known/mercure-dashboard", false},
		{"https://example.com/foo/.well-known/mercure/bar", false},

		// Relative references resolve against the hub URL.
		{"/.well-known/mercure/subscriptions/foo", true},
		{"mercure/subscriptions/foo", true},
		{"subscriptions/foo", false},
		{"bar", false},
		{"../mercure/x", false}, // resolves to /mercure/x
		{"../mercure/subscriptions/x", false},

		// Percent-encoding of unreserved characters is normalized.
		{"https://example.com/.well-known/%6Dercure/subscriptions/x", true},
		{"https://example.com/.well-known/%6dercure", true},
		{"/.well-known/me%72cure/x", true},

		// WHATWG canonicalization: backslashes are slashes in special schemes.
		{`https://example.com\.well-known\mercure\subscriptions\x`, true},

		// Opaque or non-URL topics cannot address the namespace.
		{"urn:example:mercure", false},
		{"a topic with spaces", false},

		// The empty reference resolves to the hub URL itself.
		{"", true},
	}

	for _, tc := range cases {
		t.Run(tc.topic, func(t *testing.T) {
			t.Parallel()

			base, err := wurl.Parse(urlPatternFallbackBase)
			require.NoError(t, err)

			assert.Equal(t, tc.reserved, addressesReservedNamespace(tc.topic, base), tc.topic)
		})
	}
}

func TestDecodeUnreservedPercentEncoding(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, out string
	}{
		{"", ""},
		{"/plain/path", "/plain/path"},
		{"%6D", "m"},
		{"%6d", "m"},
		{"%2F", "%2F"}, // "/" is reserved: kept encoded
		{"%2f", "%2f"}, // case preserved for non-unreserved octets
		{"%", "%"},     // truncated triplet
		{"%6", "%6"},   // truncated triplet
		{"%ZZ", "%ZZ"}, // invalid hex
		{"a%41%2Fb", "aA%2Fb"},
		{"%7E%5F%2E%2D", "~_.-"},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.out, decodeUnreservedPercentEncoding(tc.in), tc.in)
	}
}

// The guard and URL Pattern matching must resolve a path-relative topic the same
// way; different bases let a publisher forge subscription events.
func TestReservedNamespaceAgreesWithURLPatternMatching(t *testing.T) {
	t.Parallel()

	tms, err := NewTopicMatcherStore(0)
	require.NoError(t, err)

	base, err := wurl.Parse(tms.base())
	require.NoError(t, err)

	pattern := TopicMatcher{Type: MatcherTypeURLPattern, Pattern: defaultHubURL + "/subscriptions/*"}

	for _, topic := range []string{
		".well-known/mercure/subscriptions/exact/x/y",
		"./.well-known/mercure/subscriptions/exact/x/y",
		"a/../.well-known/mercure/subscriptions/exact/x/y",
	} {
		assert.Equal(t, addressesReservedNamespace(topic, base), tms.matches([]string{topic}, pattern), topic)
	}
}

func TestReservedNamespaceBaseMatchesHubURL(t *testing.T) {
	t.Parallel()

	base, err := wurl.Parse(urlPatternFallbackBase)
	require.NoError(t, err)
	assert.Equal(t, defaultHubURL, base.Pathname())
}

// Every topic resolving into the reserved namespace must fail publication validation.
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

// Topics matching reserved subscription patterns must fail validation.
func TestReservedNamespaceRejectsPatternMatchingTopics(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, base, topic string
		matcher           TopicMatcher
		deprecated        bool
	}{
		{
			name:    "a scheme equal to the configured base's resolves as a relative reference",
			base:    "https://hub.example/.well-known/mercure",
			topic:   "https:mercure/subscriptions/exact/x/y",
			matcher: TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure/subscriptions/exact/*"},
		},
		{
			name:    "a scheme and nothing else inherits the whole base path",
			base:    "https://hub.example/.well-known/mercure",
			topic:   "https:",
			matcher: TopicMatcher{Type: MatcherTypeURLPattern, Pattern: "/.well-known/mercure"},
		},
		{
			name:       "a scheme embedded in the path is not the topic's own",
			topic:      "/.well-known/mercure/subscriptions/../../https://example.com",
			matcher:    TopicMatcher{Type: deprecatedMatcherTypeName, Pattern: "/.well-known/mercure/subscriptions/{+topic}"},
			deprecated: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if c.deprecated && !deprecatedMatcherTypeCompiled() {
				t.Skip("requires the deprecated topic matcher")
			}

			tms, err := NewTopicMatcherStore(0)
			require.NoError(t, err)
			require.NoError(t, tms.setBaseURL(c.base))
			require.True(t, tms.matches([]string{c.topic}, c.matcher), "the topic must reach the reserved pattern for this case to mean anything")

			u := &Update{Topics: []string{c.topic}, Data: "forged"}
			require.ErrorIs(t, u.Validate(tms.base()), ErrReservedTopic)
		})
	}
}

// Whatever reaches a reserved pattern must fail validation with the same base.
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

func TestUpdateValidateUsesMatchingBase(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, base, topic string
		want              error
	}{
		{"same scheme", "https://hub.example/.well-known/mercure", "https:mercure/subscriptions/exact/x/y", ErrReservedTopic},
		{"different scheme", "http://hub.example/.well-known/mercure", "https:mercure/subscriptions/exact/x/y", nil},
		{"scheme only", "https://hub.example/.well-known/mercure", "https:", ErrReservedTopic},
		{"different scheme only", "http://hub.example/.well-known/mercure", "https:", nil},
		{"root base", "https://hub.example/", ".well-known/mercure/subscriptions/exact/x/y", ErrReservedTopic},
		{"hub base", "https://hub.example/.well-known/mercure", ".well-known/mercure/subscriptions/exact/x/y", nil},
		{"nested base", "https://hub.example/nested/.well-known/mercure", "../../.well-known/mercure/subscriptions/exact/x/y", ErrReservedTopic},
		{"empty base", "", "https://example.com/books/1", ErrInvalidBaseURL},
		{"relative base", "/.well-known/mercure", "https://example.com/books/1", ErrInvalidBaseURL},
		{"invalid base", "https://[", "https://example.com/books/1", ErrInvalidBaseURL},
		{"opaque base", "urn:example:hub", "https://example.com/books/1", ErrInvalidBaseURL},
		{"hostless base", "custom:/hub", "https://example.com/books/1", ErrInvalidBaseURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, topics := range [][]string{{tc.topic}, {"https://example.com/books/1", tc.topic}} {
				update := &Update{Topics: topics}
				require.ErrorIs(t, update.Validate(tc.base), tc.want)
			}
		})
	}
}

func TestPublishUsesMatchingBase(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, base, topic string
		want              error
	}{
		{"https scheme relative", "https://hub.example/.well-known/mercure", "https:mercure/subscriptions/exact/x/y", ErrReservedTopic},
		{"https scheme only", "https://hub.example/.well-known/mercure", "https:", ErrReservedTopic},
		{"http scheme relative", "http://hub.example/.well-known/mercure", "http:mercure/subscriptions/exact/x/y", ErrReservedTopic},
		{"different scheme", "http://hub.example/.well-known/mercure", "https:mercure/subscriptions/exact/x/y", nil},
		{"fallback", "", "http:mercure/subscriptions/exact/x/y", ErrReservedTopic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hub := createDummy(t, WithResourceIdentifier(tc.base))
			update := &Update{Topics: []string{"https://example.com/books/1", tc.topic}}
			require.ErrorIs(t, hub.Publish(t.Context(), update), tc.want)

			if tc.want != nil {
				assert.Empty(t, update.ID)
			}
		})
	}
}
