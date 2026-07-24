//go:build deprecated_topic && deprecated_claim

// Tests in this file cover the v8-compatibility surface:
// protocol_version_compatibility, the deprecated `topic=` query parameter,
// bare-string JWT matcher claims and the {topic}[/{subscriber}] subscription
// routes. Equivalents exercising the modern match* protocol live in
// caddy_test.go. The whole file is gated by the deprecated_topic build tag.

package caddy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Legacy bare-string-claim JWTs, accepted only under protocol_version_compatibility.
//
// publisherJWT claims: {"mercure":{"publish":["*"]}}
// subscriberJWT claims: {"mercure":{"subscribe":["*"]}}
// publisherJWTRSADeprecated: RS256 with subscribe claims containing URI templates + the
// legacy subscription URI template.
const (
	publisherJWTDeprecated    = "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.vhMwOaN5K68BTIhWokMLOeOJO4EPfT64brd8euJOA4M"
	publisherJWTRSADeprecated = "eyJhbGciOiJSUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.iwryQ5k-CWNCNQLPg7CtgTdDWbG_CurSxDK8kMjTZfprGhh7Yli1SFt8WB3U4zbZ2wxUO7UfprZq3hnl8nSrozO9KDTCDwCYhMgRlcrdwm6XL1uXFwMJt4VSmp1srCQotv0FgT11jF8Km1vMQQOnUC27Va9fbfRtITVsjxsveYeMJqusVWO6F3vAvkM35oL8E8qgBbfrG_lnuhb_9Ws6RIq4YOslkOar_gopEs00CITxmV_aHVHRYzeW7QpycxjC7m8Mp-lKzaUewvJuKWI5HsM134xfaH8RAHSvh6H9pVQAiJ9tyc17bAx46M98WMsHFokVwz3rd7PoGGou6A7y5RzeGpiSxykTWCPPcBnxJ1gwUYqEYGTnRjl9JmhHY_VfQP4edyU-zhmMCCSie8rvkRDilAQGd5kj5m1voSn-EqA13sSe69evXxVUIB2nO70qHCcHBBHxunLqTIIerpc3F9_WWM4_Q_0j9CoTd2aFyuq_sdc6RcmAE3uTznp2DyKNQkT1EfpY7xCCe1MR-Webez5Ioa1EMDP0KrvLdnNRmuM3THSu1pqcvPV7Di7dJci5QWsYEmaP8cLuuZXdAhy_UoSgzbvfT_8mlDoJ9VvDXLJ39OwGYIyZiZ9VTNXm8mxre993cqg7boZRS8x70VRxnjmNxm40SgEvb6CHYO0lSBU"
	subscriberJWTDeprecated   = "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyIqIl19fQ.g3w81T7YQLKLrgovor9uEKUiOCAx6DmAAbq18qmDwsY"
)

func TestMercureDeprecated(t *testing.T) {
	boltPath := filepath.Join(t.TempDir(), "bolt.db")

	data := []struct {
		name            string
		transportConfig string
	}{
		{"bolt", `transport bolt {
			path ` + boltPath + `
		}`},
		{"local", "transport local\n"},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			if d.name == "bolt" {
				t.Cleanup(func() {
					require.NoError(t, os.Remove(boltPath))
				})
			}

			tester := caddytest.NewTester(t)
			tester.InitServer(fmt.Sprintf(`{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			protocol_version_compatibility 8
			%[1]s
		}

		respond 404
	}
}

example.com:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			protocol_version_compatibility 8
			%[1]s
		}

		respond 404
	}
}`, d.transportConfig), "caddyfile")

			var connected, received sync.WaitGroup

			connected.Add(1)
			received.Go(func() {
				cx, cancel := context.WithCancel(t.Context())
				req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?topic=https%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
				req = req.WithContext(cx)
				resp := tester.AssertResponseCode(req, http.StatusOK)

				connected.Done()

				var receivedBody strings.Builder

				buf := make([]byte, 1024)
				for {
					_, err := resp.Body.Read(buf)
					require.NoError(t, err)

					receivedBody.Write(buf)

					if strings.Contains(receivedBody.String(), "data: bar\n") {
						cancel()

						break
					}
				}

				assert.NoError(t, resp.Body.Close())
			})

			connected.Wait()

			body := url.Values{"topic": {"https://example.com/foo/1"}, "data": {"bar"}, "id": {"bar"}}
			req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
			require.NoError(t, err)
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Add("Authorization", bearerPrefix+publisherJWTDeprecated)

			resp := tester.AssertResponseCode(req, http.StatusOK)
			require.NoError(t, resp.Body.Close())

			received.Wait()

			if d.name != "bolt" {
				assert.NoFileExists(t, boltPath)
			}
		})
	}
}

func TestJWTPlaceholdersDeprecated(t *testing.T) {
	k, _ := os.ReadFile("../fixtures/jwt/RS256.key.pub")
	t.Setenv("TEST_JWT_KEY", string(k))
	t.Setenv("TEST_JWT_ALG", "RS256")

	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port     9080
		https_port    9443
	}

	localhost:9080 {
		route {
			mercure {
				anonymous
				publisher_jwt {env.TEST_JWT_KEY} {env.TEST_JWT_ALG}
				protocol_version_compatibility 8
				transport local
			}

			respond 404
		}
	}
	`, "caddyfile")

	var connected, received sync.WaitGroup

	connected.Add(1)
	received.Go(func() {
		cx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?topic=https%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req = req.WithContext(cx)
		resp := tester.AssertResponseCode(req, http.StatusOK)

		connected.Done()

		var receivedBody strings.Builder

		buf := make([]byte, 1024)
		for {
			_, err := resp.Body.Read(buf)
			require.NoError(t, err)

			receivedBody.Write(buf)

			if strings.Contains(receivedBody.String(), "data: bar\n") {
				cancel()

				break
			}
		}

		assert.NoError(t, resp.Body.Close())
	})

	connected.Wait()

	body := url.Values{"topic": {"https://example.com/foo/1"}, "data": {"bar"}, "id": {"bar"}}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+publisherJWTRSADeprecated)

	resp := tester.AssertResponseCode(req, http.StatusOK)
	require.NoError(t, resp.Body.Close())

	received.Wait()
}

func TestSubscriptionAPIDeprecated(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port     9080
		https_port    9443
	}

	localhost:9080 {
		route {
			mercure {
				anonymous
				subscriptions
				publisher_jwt !ChangeMe!
				protocol_version_compatibility 8
			}

			respond 404
		}
	}
	`, "caddyfile")

	req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure/subscriptions", nil)
	resp := tester.AssertResponseCode(req, http.StatusOK)
	require.NoError(t, resp.Body.Close())
}

func TestCookieNameDeprecated(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		skip_install_trust
		admin localhost:2999
		http_port     9080
		https_port    9443
	}
	localhost:9080 {
		route {
			mercure {
				publisher_jwt !ChangeMe!
				subscriber_jwt !ChangeMe!
				cookie_name foo
				publish_origins http://localhost:9080
				protocol_version_compatibility 8
			}

			respond 404
		}
	}
	`, "caddyfile")

	var connected, received sync.WaitGroup

	connected.Add(1)
	received.Go(func() {
		cx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?topic=https%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req.Header.Add("Origin", "http://localhost:9080")
		req.AddCookie(&http.Cookie{Name: "foo", Value: subscriberJWTDeprecated})
		req = req.WithContext(cx)
		resp := tester.AssertResponseCode(req, http.StatusOK)

		connected.Done()

		var receivedBody strings.Builder

		buf := make([]byte, 1024)
		for {
			_, err := resp.Body.Read(buf)
			require.NoError(t, err)

			receivedBody.Write(buf)

			if strings.Contains(receivedBody.String(), "data: bar\n") {
				cancel()

				break
			}
		}

		assert.NoError(t, resp.Body.Close())
	})

	connected.Wait()

	body := url.Values{"topic": {"https://example.com/foo/1"}, "data": {"bar"}, "id": {"bar"}, "private": {"1"}}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Origin", "http://localhost:9080")
	req.AddCookie(&http.Cookie{Name: "foo", Value: publisherJWTDeprecated})

	resp := tester.AssertResponseCode(req, http.StatusOK)
	require.NoError(t, resp.Body.Close())

	received.Wait()
}

func TestBoltConfigDeprecated(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, os.Remove("test.db"))
	})

	tester := caddytest.NewTester(t)
	tester.InitServer(`
{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			protocol_version_compatibility 8
			transport bolt {
				path test.db
				bucket_name foo
				size 20
				cleanup_frequency 0.2
			}
		}

		respond 404
	}
}`, "caddyfile")

	assert.FileExists(t, "test.db")
}

// The deprecated top-level publisher_jwt directive maps to the implicit issuer,
// which is rejected in modern mode. Without protocol_version_compatibility the
// hub must auto-enable compatibility mode so a 0.x bare-claim token still
// publishes, instead of failing to provision.
func TestMercureDeprecatedAutoCompat(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`{
	skip_install_trust
	admin localhost:2999
	http_port     9080
	https_port    9443
}

localhost:9080 {
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
			transport local
		}

		respond 404
	}
}`, "caddyfile")

	var connected, received sync.WaitGroup

	connected.Add(1)
	received.Go(func() {
		cx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?topic=https%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req = req.WithContext(cx)
		resp := tester.AssertResponseCode(req, http.StatusOK)

		connected.Done()

		var receivedBody strings.Builder

		buf := make([]byte, 1024)
		for {
			_, err := resp.Body.Read(buf)
			require.NoError(t, err)

			receivedBody.Write(buf)

			if strings.Contains(receivedBody.String(), "data: bar\n") {
				cancel()

				break
			}
		}

		assert.NoError(t, resp.Body.Close())
	})

	connected.Wait()

	body := url.Values{"topic": {"https://example.com/foo/1"}, "data": {"bar"}, "id": {"bar"}}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+publisherJWTDeprecated)

	resp := tester.AssertResponseCode(req, http.StatusOK)
	require.NoError(t, resp.Body.Close())

	received.Wait()
}
