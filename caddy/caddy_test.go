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

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bearerPrefix = "Bearer "

	// Object-form JWT fixtures used by the non-deprecated tests.
	//
	// publisherJWT claims: {"mercure":{"publish":[{"match":"*"}]}}
	// subscriberJWT claims: {"mercure":{"subscribe":[{"match":"*"}]}}.
	publisherJWT  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XX19.fxYhQH3ML8SA0ZYSo8qVUUezvIO6O6JNDRF5RN3zeZU"
	subscriberJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6W3sibWF0Y2giOiIqIn1dfX0.73III1dtzX0DfOWGIeDbNCq0dmcyfCt4XEcUMQNzt-w"

	// publisherJWTRSA: RS256 object-form JWT signed with fixtures/jwt/RS256.key.
	// Claims: {"mercure":{"publish":[{"match":"*"}],"subscribe":[
	//   {"match":"https://example.com/my-private-topic"},
	//   {"match":"https://example.com/demo/books/:id.jsonld","matchType":"URLPattern"}
	// ],"payload":{"user":"https://example.com/users/dunglas","remoteAddr":"127.0.0.1"}}}
	publisherJWTRSA = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlt7Im1hdGNoIjoiKiJ9XSwic3Vic2NyaWJlIjpbeyJtYXRjaCI6Imh0dHBzOi8vZXhhbXBsZS5jb20vbXktcHJpdmF0ZS10b3BpYyJ9LHsibWF0Y2giOiJodHRwczovL2V4YW1wbGUuY29tL2RlbW8vYm9va3MvOmlkLmpzb25sZCIsIm1hdGNoVHlwZSI6IlVSTFBhdHRlcm4ifV0sInBheWxvYWQiOnsicmVtb3RlQWRkciI6IjEyNy4wLjAuMSIsInVzZXIiOiJodHRwczovL2V4YW1wbGUuY29tL3VzZXJzL2R1bmdsYXMifX19.ZDg48hUAzGEyiWyw4jl-vVPcdF4Mg9Lk22BKZCPXvHsRrDK9UZW-LaV9sEnJ82v80gHm_4xbgUqREPuN0aD_F16K6TaGHxAkjOggQKWByG-7l77mhUUTa7UQtR6HvHcWcgaswCcs6LBFqnSpQ-BSsHgeRJmlXqq_r3xJrLzezaOlQVqS1rDy1fAIdva3dzYTOwdji7M5a4gSDES-D05TETlOhQp1Cg7yxs2elL13n8j0-BdIbY8SkO1vy3GtHqqHWpj3pB9ks-D_VQwJQuLAOaXJKG5sVKLOG1EsgX_fYRbryWwgZpPO_IjHiL6y0bz5CWjjYBYfqOL3hUYCBQXp2A3J02CctvDHqgmlorxhCaA6GkKV-LXqP2tQNSiOMBl6TjCxQhKnou9lK27W2jsxXD5TRx3jmStJ38J1wT99HRGWOzJ9re3HlwPv1NsgNPn3kJdg9OSwWaxx_PqLbWSoHA09F66e4eMgaxZT_16HzbZZAymy9MsBrcCM7C-JyHnUZ97YjwuGm6MQDtvQWuTkixkSxHCpsv6EmbqJc-4cp9tP5ZeFYcZQTyu2jkQrvNzca-8GunXGftfH-IxckPoTwREd2wywwI3ZcRTKZ0SBd3iq8Jnxdgmu22dgCdrOFkM4lMn5x7rVT_fxUuF5EYTeTxkil1LcmPf17r4eNAImxfU"
)

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
