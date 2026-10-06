//go:build !deprecated_topic

package mercure

import (
	"testing"

	wurl "github.com/nlnwa/whatwg-url/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The raw-path test exists only for the v8 matcher, whose templates match a raw
// string resolution never produces. Without it compiled in, a path that leaves the
// reserved namespace once normalized is genuinely outside it and stays publishable:
// the guard is deliberately narrower here.
func TestReservedNamespaceIgnoresDotSegments(t *testing.T) {
	t.Parallel()

	base, err := wurl.Parse(urlPatternFallbackBase)
	require.NoError(t, err)

	for _, topic := range []string{
		"https://example.com/.well-known/mercure/subscriptions/../..",
		"https://example.com/.well-known/mercure/../books/1",
		"/.well-known/mercure/subscriptions/../..",
	} {
		assert.False(t, addressesReservedNamespace(topic, base), topic)
	}
}
