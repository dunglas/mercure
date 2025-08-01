package caddy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bearerPrefix    = "Bearer "
	publisherJWT    = "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.vhMwOaN5K68BTIhWokMLOeOJO4EPfT64brd8euJOA4M"
	publisherJWTRSA = "eyJhbGciOiJSUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.iwryQ5k-CWNCNQLPg7CtgTdDWbG_CurSxDK8kMjTZfprGhh7Yli1SFt8WB3U4zbZ2wxUO7UfprZq3hnl8nSrozO9KDTCDwCYhMgRlcrdwm6XL1uXFwMJt4VSmp1srCQotv0FgT11jF8Km1vMQQOnUC27Va9fbfRtITVsjxsveYeMJqusVWO6F3vAvkM35oL8E8qgBbfrG_lnuhb_9Ws6RIq4YOslkOar_gopEs00CITxmV_aHVHRYzeW7QpycxjC7m8Mp-lKzaUewvJuKWI5HsM134xfaH8RAHSvh6H9pVQAiJ9tyc17bAx46M98WMsHFokVwz3rd7PoGGou6A7y5RzeGpiSxykTWCPPcBnxJ1gwUYqEYGTnRjl9JmhHY_VfQP4edyU-zhmMCCSie8rvkRDilAQGd5kj5m1voSn-EqA13sSe69evXxVUIB2nO70qHCcHBBHxunLqTIIerpc3F9_WWM4_Q_0j9CoTd2aFyuq_sdc6RcmAE3uTznp2DyKNQkT1EfpY7xCCe1MR-Webez5Ioa1EMDP0KrvLdnNRmuM3THSu1pqcvPV7Di7dJci5QWsYEmaP8cLuuZXdAhy_UoSgzbvfT_8mlDoJ9VvDXLJ39OwGYIyZiZ9VTNXm8mxre993cqg7boZRS8x70VRxnjmNxm40SgEvb6CHYO0lSBU"
	subscriberJWT   = "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyIqIl19fQ.g3w81T7YQLKLrgovor9uEKUiOCAx6DmAAbq18qmDwsY"
)

func TestMercure(t *testing.T) {
	data := []struct {
		name            string
		transportConfig string
	}{
		{"bolt", ""},
		{"local", "transport local\n"},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			if d.name == "bolt" {
				t.Cleanup(func() {
					require.NoError(t, os.Remove("bolt.db"))
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

			var connected sync.WaitGroup
			var received sync.WaitGroup
			connected.Add(1)
			received.Add(1)

			go func() {
				cx, cancel := context.WithCancel(t.Context())
				req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
				req = req.WithContext(cx)
				resp := tester.AssertResponseCode(req, http.StatusOK)

				connected.Done()

				var receivedBody strings.Builder
				buf := make([]byte, 1024)
				for {
					_, err := resp.Body.Read(buf)
					if errors.Is(err, io.EOF) {
						panic("EOF")
					}

					receivedBody.Write(buf)
					if strings.Contains(receivedBody.String(), "data: bar\n") {
						cancel()

						break
					}
				}

				resp.Body.Close()
				received.Done()
			}()

			connected.Wait()

			body := url.Values{"topic": {"http://example.com/foo/1"}, "data": {"bar"}, "id": {"bar"}}
			req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
			require.NoError(t, err)
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Add("Authorization", bearerPrefix+publisherJWT)

			resp := tester.AssertResponseCode(req, http.StatusOK)
			resp.Body.Close()

			received.Wait()

			if d.name != "bolt" {
				assert.NoFileExists(t, "bolt.db")
			}
		})
	}
}

func TestJWTPlaceholders(t *testing.T) {
	defer os.Remove("bolt.db")

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
			}
	
			respond 404
		}
	}
	`, "caddyfile")

	var connected sync.WaitGroup
	var received sync.WaitGroup
	connected.Add(1)
	received.Add(1)

	go func() {
		cx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req = req.WithContext(cx)
		resp := tester.AssertResponseCode(req, http.StatusOK)

		connected.Done()

		var receivedBody strings.Builder
		buf := make([]byte, 1024)
		for {
			_, err := resp.Body.Read(buf)
			if errors.Is(err, io.EOF) {
				panic("EOF")
			}

			receivedBody.Write(buf)
			if strings.Contains(receivedBody.String(), "data: bar\n") {
				cancel()

				break
			}
		}

		resp.Body.Close()
		received.Done()
	}()

	connected.Wait()

	body := url.Values{"topic": {"http://example.com/foo/1"}, "data": {"bar"}, "id": {"bar"}}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+publisherJWTRSA)

	resp := tester.AssertResponseCode(req, http.StatusOK)
	resp.Body.Close()

	received.Wait()
}

func TestSubscriptionAPI(t *testing.T) {
	defer os.Remove("bolt.db")

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
	resp.Body.Close()
}

func TestCookieName(t *testing.T) {
	defer os.Remove("bolt.db")

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

	var connected sync.WaitGroup
	var received sync.WaitGroup
	connected.Add(1)
	received.Add(1)

	go func() {
		cx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9080/.well-known/mercure?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req.Header.Add("Origin", "http://localhost:9080")
		req.AddCookie(&http.Cookie{Name: "foo", Value: subscriberJWT})
		req = req.WithContext(cx)
		resp := tester.AssertResponseCode(req, http.StatusOK)

		connected.Done()

		var receivedBody strings.Builder
		buf := make([]byte, 1024)
		for {
			_, err := resp.Body.Read(buf)
			if errors.Is(err, io.EOF) {
				panic("EOF")
			}

			receivedBody.Write(buf)
			if strings.Contains(receivedBody.String(), "data: bar\n") {
				cancel()

				break
			}
		}

		resp.Body.Close()
		received.Done()
	}()

	connected.Wait()

	body := url.Values{"topic": {"http://example.com/foo/1"}, "data": {"bar"}, "id": {"bar"}, "private": {"1"}}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode()))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Origin", "http://localhost:9080")
	req.AddCookie(&http.Cookie{Name: "foo", Value: publisherJWT})

	resp := tester.AssertResponseCode(req, http.StatusOK)
	resp.Body.Close()

	received.Wait()
}

func TestBoltConfig(t *testing.T) {
	defer os.Remove("test.db")

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
									"subscriber_jwt": {},
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
									"subscriber_jwt": {},
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
