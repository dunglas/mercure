package caddy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bearerPrefix = "Bearer "

	// caddyResourceIdentifier is the access-token audience the test Caddyfiles
	// configure via `resource_identifier`.
	caddyResourceIdentifier = "https://example.com/.well-known/mercure"

	// caddyTrustedIssuer is the access-token issuer the test Caddyfiles
	// configure via `trusted_issuers`.
	caddyTrustedIssuer = "https://example.com"
)

// RFC 9068 access tokens used by the non-deprecated tests, minted at package
// init so the assertions stay readable.
//
//nolint:gochecknoglobals
var (
	// publisherJWT grants publish on every topic (HS256, key "!ChangeMe!").
	publisherJWT = mustMintHMACToken(actionDetail("publish", topicMatch("*")))
	// subscriberJWT grants subscribe on every topic (HS256, key "!ChangeMe!").
	subscriberJWT = mustMintHMACToken(actionDetail("subscribe", topicMatch("*")))
	// publisherJWTRSA grants publish on every topic, signed with
	// fixtures/jwt/RS256.key, to exercise RS256 verification.
	publisherJWTRSA = mustMintRSAToken(actionDetail("publish", topicMatch("*")))
)

func topicMatch(match string) map[string]any { return map[string]any{"match": match} }

func actionDetail(action string, topics ...map[string]any) map[string]any {
	return map[string]any{"type": "https://mercure.rocks/authorization-detail", "actions": []string{action}, "topics": topics}
}

func newAccessTokenClaims(details ...map[string]any) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":                   caddyTrustedIssuer,
		"aud":                   caddyResourceIdentifier,
		"exp":                   time.Now().Add(time.Hour).Unix(),
		"authorization_details": details,
	}
}

func mustMintHMACToken(details ...map[string]any) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, newAccessTokenClaims(details...))
	token.Header["typ"] = "at+jwt"

	s, err := token.SignedString([]byte("!ChangeMe!"))
	if err != nil {
		panic(err)
	}

	return s
}

func mustMintRSAToken(details ...map[string]any) string {
	pem, err := os.ReadFile("../fixtures/jwt/RS256.key")
	if err != nil {
		panic(err)
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(pem)
	if err != nil {
		panic(err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, newAccessTokenClaims(details...))
	token.Header["typ"] = "at+jwt"

	s, err := token.SignedString(key)
	if err != nil {
		panic(err)
	}

	return s
}

func TestMercure(t *testing.T) {
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
			issuer https://example.com {
				publisher {
					jwt !ChangeMe!
				}
			}
			resource_identifier https://example.com/.well-known/mercure
			%[1]s
		}

		respond 404
	}
}

example.com:9080 {
	route {
		mercure {
			anonymous
			issuer https://example.com {
				publisher {
					jwt !ChangeMe!
				}
			}
			resource_identifier https://example.com/.well-known/mercure
			%[1]s
		}

		respond 404
	}
}`, d.transportConfig), "caddyfile")

			var connected, received sync.WaitGroup

			connected.Add(1)
			received.Go(func() {
				cx, cancel := context.WithCancel(t.Context())
				req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?match=https%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
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
			req.Header.Add("Authorization", bearerPrefix+publisherJWT)

			resp := tester.AssertResponseCode(req, http.StatusOK)
			require.NoError(t, resp.Body.Close())

			received.Wait()

			if d.name != "bolt" {
				assert.NoFileExists(t, boltPath)
			}
		})
	}
}

// TestJWTPlaceholders exercises env-var placeholder support with an object-form
// RS256 JWT. The deprecated URI-template subscribe claim lives in
// TestJWTPlaceholdersDeprecated — here the publisher uses the modern
// "publish all topics" form with a URL-pattern subscribe claim.
func TestJWTPlaceholders(t *testing.T) {
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
				issuer https://example.com {
					publisher {
						jwt {env.TEST_JWT_KEY} {env.TEST_JWT_ALG}
					}
				}
				resource_identifier https://example.com/.well-known/mercure
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
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?match=https%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
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
	req.Header.Add("Authorization", bearerPrefix+publisherJWTRSA)

	resp := tester.AssertResponseCode(req, http.StatusOK)
	require.NoError(t, resp.Body.Close())

	received.Wait()
}

func TestSubscriptionAPI(t *testing.T) {
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
				issuer https://example.com {
					publisher {
						jwt !ChangeMe!
					}
				}
				resource_identifier https://example.com/.well-known/mercure
			}

			respond 404
		}
	}
	`, "caddyfile")

	req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure/subscriptions", nil)
	resp := tester.AssertResponseCode(req, http.StatusOK)
	require.NoError(t, resp.Body.Close())
}

func TestCookieName(t *testing.T) {
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
				issuer https://example.com {
					publisher {
						jwt !ChangeMe!
					}
					subscriber {
						jwt !ChangeMe!
					}
				}
				resource_identifier https://example.com/.well-known/mercure
				cookie_name foo
				publish_origins http://localhost:9080
			}

			respond 404
		}
	}
	`, "caddyfile")

	var connected, received sync.WaitGroup

	connected.Add(1)
	received.Go(func() {
		cx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?match=https%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req.Header.Add("Origin", "http://localhost:9080")
		req.AddCookie(&http.Cookie{Name: "foo", Value: subscriberJWT})
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
	req.AddCookie(&http.Cookie{Name: "foo", Value: publisherJWT})

	resp := tester.AssertResponseCode(req, http.StatusOK)
	require.NoError(t, resp.Body.Close())

	received.Wait()
}

func TestProtectedResourceMetadata(t *testing.T) {
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
				issuer https://as.example.com {
					authorization_server
					publisher {
						jwt !ChangeMe!
					}
					subscriber {
						jwt !ChangeMe!
					}
				}
				resource_identifier https://example.com/.well-known/mercure
			}

			respond 404
		}
	}
	`, "caddyfile")

	req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/oauth-protected-resource/.well-known/mercure", nil)

	resp := tester.AssertResponseCode(req, http.StatusOK)
	defer resp.Body.Close()

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Contains(t, string(b), `"resource":"https://example.com/.well-known/mercure"`)
	assert.Contains(t, string(b), `"authorization_servers":["https://as.example.com"]`)
}

// The protocol requires rejecting requests exceeding the body-size limit with
// a 413 status code.
func TestMaxRequestBodySize(t *testing.T) {
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
				issuer https://example.com {
					publisher {
						jwt !ChangeMe!
					}
					subscriber {
						jwt !ChangeMe!
					}
				}
				resource_identifier https://example.com/.well-known/mercure
				max_request_body_size 1KB
				transport local
			}

			respond 404
		}
	}
	`, "caddyfile")

	body := url.Values{"topic": {"https://example.com/foo/1"}, "data": {strings.Repeat("x", 2048)}}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+publisherJWT)

	resp := tester.AssertResponseCode(req, http.StatusRequestEntityTooLarge)
	require.NoError(t, resp.Body.Close())
}

func TestAllowNoPublish(t *testing.T) {
	AllowNoPublish = true

	t.Cleanup(func() {
		AllowNoPublish = false
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
				issuer https://example.com {
					subscriber {
						jwt !ChangeMe!
					}
				}
				resource_identifier https://example.com/.well-known/mercure
			}

			respond 404
		}
	}
	`, "caddyfile")

	req, _ := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", nil)
	r := tester.AssertResponseCode(req, http.StatusMethodNotAllowed)
	require.NoError(t, r.Body.Close())
}

func TestBoltConfig(t *testing.T) {
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
			issuer https://example.com {
				publisher {
					jwt !ChangeMe!
				}
			}
			resource_identifier https://example.com/.well-known/mercure
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

func TestAdaptBoltConfig(t *testing.T) {
	caddytest.AssertAdapt(t, `http://

mercure {
	publisher_jwt !ChangeMe!
	transport bolt {
		path test.db
		bucket_name foo
		size 20
		cleanup_frequency 0.2
	}
}
`, "caddyfile", `{
	"apps": {
		"http": {
			"servers": {
				"srv0": {
					"listen": [
						":80"
					],
					"routes": [
						{
							"handle": [
								{
									"handler": "mercure",
									"publisher_jwt": {
										"key": "!ChangeMe!"
									},
									"transport": {
										"bucket_name": "foo",
										"cleanup_frequency": 0.2,
										"name": "bolt",
										"path": "test.db",
										"size": 20
									}
								}
							]
						}
					]
				}
			}
		}
	}
}`)
}

func TestAdaptLocalConfig(t *testing.T) {
	caddytest.AssertAdapt(t, `http://

mercure {
	publisher_jwt !ChangeMe!
	transport local
}
`, "caddyfile", `{
	"apps": {
		"http": {
			"servers": {
				"srv0": {
					"listen": [
						":80"
					],
					"routes": [
						{
							"handle": [
								{
									"handler": "mercure",
									"publisher_jwt": {
										"key": "!ChangeMe!"
									},
									"transport": {
										"name": "local"
									}
								}
							]
						}
					]
				}
			}
		}
	}
}`)
}

func TestNewJWKSetKeyfunc(t *testing.T) {
	jwksPath, err := filepath.Abs("testdata/RS256.jwks.json")
	require.NoError(t, err)

	t.Run("file URL with empty host", func(t *testing.T) {
		k, err := newJWKSetKeyfunc(t.Context(), "file://"+jwksPath)
		require.NoError(t, err)
		assert.NotNil(t, k)
	})

	t.Run("file URL with localhost host", func(t *testing.T) {
		k, err := newJWKSetKeyfunc(t.Context(), "file://localhost"+jwksPath)
		require.NoError(t, err)
		assert.NotNil(t, k)
	})

	t.Run("file URL with rejected host", func(t *testing.T) {
		_, err := newJWKSetKeyfunc(t.Context(), "file://example.com"+jwksPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"example.com"`)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := newJWKSetKeyfunc(t.Context(), "file://"+filepath.Join(t.TempDir(), "absent.json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read JWK Set file")
	})

	t.Run("invalid JWK Set JSON", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(bad, []byte("not json"), 0o600))

		_, err := newJWKSetKeyfunc(t.Context(), "file://"+bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JWK Set file")
	})
}

// TestMultiIssuerPublish exercises per-issuer key binding through the Caddy
// module: two issuers with distinct keys, each verified only with its own key.
func TestMultiIssuerPublish(t *testing.T) {
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
				issuer https://a.example {
					publisher {
						jwt key-a
					}
				}
				issuer https://b.example {
					publisher {
						jwt key-b
					}
				}
				resource_identifier https://example.com/.well-known/mercure
				transport local
			}

			respond 404
		}
	}
	`, "caddyfile")

	mint := func(key []byte, iss string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss":                   iss,
			"aud":                   caddyResourceIdentifier,
			"exp":                   time.Now().Add(time.Hour).Unix(),
			"authorization_details": []map[string]any{actionDetail("publish", topicMatch("*"))},
		})
		token.Header["typ"] = "at+jwt"

		s, err := token.SignedString(key)
		require.NoError(t, err)

		return s
	}

	publish := func(tokenStr string) *http.Request {
		body := url.Values{"topic": {"https://example.com/foo"}, "data": {"hi"}}
		req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
		require.NoError(t, err)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Authorization", bearerPrefix+tokenStr)

		return req
	}

	// Issuer A signed with key A is accepted.
	resp := tester.AssertResponseCode(publish(mint([]byte("key-a"), "https://a.example")), http.StatusOK)
	require.NoError(t, resp.Body.Close())

	// Issuer A signed with key B is rejected: keys are not pooled across issuers.
	resp = tester.AssertResponseCode(publish(mint([]byte("key-b"), "https://a.example")), http.StatusUnauthorized)
	require.NoError(t, resp.Body.Close())
}
