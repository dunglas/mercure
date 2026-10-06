//go:build deprecated_claim

package mercure

import (
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestPublishHandlerLegacyClaimV7(t *testing.T) {
	t.Parallel()

	legacyToken := func(t *testing.T, mercure map[string]any) string {
		t.Helper()

		c := jwt.MapClaims{}
		if mercure != nil {
			c["mercure"] = mercure
		}

		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte("publisher"))
		require.NoError(t, err)

		return s
	}

	matchers := func(patterns ...string) []map[string]string {
		m := make([]map[string]string, len(patterns))
		for i, p := range patterns {
			m[i] = map[string]string{"match": p}
		}

		return m
	}

	for _, tc := range []struct {
		name    string
		mode    int
		mercure map[string]any
		private bool
		status  int
	}{
		{"no mercure claim", 7, nil, false, http.StatusForbidden},
		{"missing publish", 7, map[string]any{"subscribe": matchers("https://example.com/books/1")}, false, http.StatusForbidden},
		{"null publish", 7, map[string]any{"publish": nil}, false, http.StatusForbidden},
		{"empty publish", 7, map[string]any{"publish": []any{}}, false, http.StatusOK},
		{"empty publish private", 7, map[string]any{"publish": []any{}}, true, http.StatusForbidden},
		{"other topic", 7, map[string]any{"publish": matchers("https://example.com/other")}, false, http.StatusOK},
		{"other topic private", 7, map[string]any{"publish": matchers("https://example.com/other")}, true, http.StatusForbidden},
		{"granted topic private", 7, map[string]any{"publish": matchers("https://example.com/books/1")}, true, http.StatusOK},
		{"v8 empty publish", 8, map[string]any{"publish": []any{}}, false, http.StatusForbidden},
		{"v8 other topic", 8, map[string]any{"publish": matchers("https://example.com/other")}, false, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hub := createDummy(t, WithProtocolVersionCompatibility(tc.mode))
			assertPublishStatus(t, hub, legacyToken(t, tc.mercure), tc.private, tc.status)
		})
	}
}
