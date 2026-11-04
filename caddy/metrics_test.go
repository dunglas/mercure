package caddy

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsOfSeveralHubs checks that the mercure_* series sum all the hubs of
// a configuration and that every hub reports its own mercure_hub_* series
// (https://github.com/dunglas/mercure/issues/1457).
func TestMetricsOfSeveralHubs(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port     9080
		https_port    9443
		metrics
	}

	http://localhost:9080 {
		mercure {
			name hub-a
			transport local
			anonymous
			issuer https://example.com {
				publisher {
					jwt !ChangeMe!
				}
			}
			resource_identifier https://example.com/.well-known/mercure
		}
	}

	http://localhost:9081 {
		mercure {
			name hub-b
			transport local
			anonymous
			issuer https://example.com {
				publisher {
					jwt !ChangeMe!
				}
			}
			resource_identifier https://example.com/.well-known/mercure
		}
	}
	`, "caddyfile")

	for hubURL, updates := range map[string]int{"http://localhost:9080": 1, "http://localhost:9081": 2} {
		for range updates {
			body := url.Values{"topic": {"https://example.com/foo"}, "data": {"bar"}}
			req, err := http.NewRequest(http.MethodPost, hubURL+"/.well-known/mercure", strings.NewReader(body.Encode()))
			require.NoError(t, err)
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Add("Authorization", bearerPrefix+publisherJWT)

			resp := tester.AssertResponseCode(req, http.StatusOK)
			require.NoError(t, resp.Body.Close())
		}
	}

	req, err := http.NewRequest(http.MethodGet, "http://localhost:2999/metrics", nil)
	require.NoError(t, err)

	resp := tester.AssertResponseCode(req, http.StatusOK)
	metrics, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Contains(t, string(metrics), "\nmercure_updates_total 3\n")
	assert.Contains(t, string(metrics), `mercure_hub_updates_total{hub="hub-a"} 1`+"\n")
	assert.Contains(t, string(metrics), `mercure_hub_updates_total{hub="hub-b"} 2`+"\n")
}
