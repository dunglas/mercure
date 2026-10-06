package mercure

import (
	"testing"

	wurl "github.com/nlnwa/whatwg-url/url"
	"github.com/stretchr/testify/require"
)

var benchSink bool

func BenchmarkAddressesReservedNamespace(b *testing.B) {
	base, err := wurl.Parse(urlPatternFallbackBase)
	require.NoError(b, err)

	for _, c := range []struct{ name, topic string }{
		{"AbsoluteURL", "https://example.com/books/1"},
		{"AbsoluteDeep", "https://example.com/users/dunglas/very/deep/resource/tree/42"},
		{"Relative", "/books/1"},
		{"Wildcard", "*"},
		{"Reserved", "https://example.com/.well-known/mercure"},
	} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				benchSink = addressesReservedNamespace(c.topic, base)
			}
		})
	}
}
