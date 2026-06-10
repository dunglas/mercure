package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

			assert.Equal(t, tc.reserved, addressesReservedNamespace(tc.topic), tc.topic)
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
