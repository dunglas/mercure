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
	return map[string]any{"type": "mercure", "actions": []string{action}, "topics": topics}
}

func newAccessTokenClaims(details ...map[string]any) jwt.MapClaims {
	return jwt.MapClaims{
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
			publisher_jwt !ChangeMe!
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
			publisher_jwt !ChangeMe!
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
				publisher_jwt {env.TEST_JWT_KEY} {env.TEST_JWT_ALG}
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
				publisher_jwt !ChangeMe!
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
				publisher_jwt !ChangeMe!
				subscriber_jwt !ChangeMe!
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
				publisher_jwt !ChangeMe!
				subscriber_jwt !ChangeMe!
				resource_identifier https://example.com/.well-known/mercure
				authorization_servers https://as.example.com
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
				subscriber_jwt !ChangeMe!
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
			publisher_jwt !ChangeMe!
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
}
