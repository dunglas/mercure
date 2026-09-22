//go:build deprecated_topic

package mercure

import (
	"testing"

	wurl "github.com/nlnwa/whatwg-url/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The v8 matcher compares raw strings, so a path that only leaves the namespace
// once normalized must still be refused at publication.
func TestReservedNamespaceRejectsDotSegments(t *testing.T) {
	t.Parallel()

	base, err := wurl.Parse(urlPatternFallbackBase)
	require.NoError(t, err)

	for _, topic := range []string{
		"https://example.com/.well-known/mercure/subscriptions/../..",
		"https://example.com/.well-known/mercure/../books/1",
		"/.well-known/mercure/subscriptions/../..",
	} {
		assert.True(t, addressesReservedNamespace(topic, base), topic)
	}

	for _, topic := range []string{
		"https://example.com/books/1",
		"https://example.com/well-known/mercure",
		"https://example.com/books/1?next=/.well-known/mercure",
	} {
		assert.False(t, addressesReservedNamespace(topic, base), topic)
	}
}
