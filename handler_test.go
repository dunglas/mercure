package mercure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// rs/cors treats any "*" in an allowed origin as a wildcard and reflects the
// request's Origin, which a browser accepts alongside credentials. Only a
// wildcard confined to the subdomain labels of one host keeps the response
// bound to an allowlist.
func TestSpansArbitraryOrigins(t *testing.T) {
	t.Parallel()

	for origin, spans := range map[string]bool{
		"https://example.com":   false,
		"https://*.example.com": false,
		"null":                  false,
		"*":                     true,
		"https://*":             true,
		"https://*example.com":  true,
		"http*://example.com":   true,
		"https://*.*.com":       true,
	} {
		assert.Equal(t, spans, spansArbitraryOrigins(origin), origin)
	}
}
