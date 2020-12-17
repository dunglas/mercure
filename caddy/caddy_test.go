package caddy

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/require"
)

const (
	publisherJWT    = "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.vhMwOaN5K68BTIhWokMLOeOJO4EPfT64brd8euJOA4M"
	publisherJWTRSA = "eyJhbGciOiJSUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.iwryQ5k-CWNCNQLPg7CtgTdDWbG_CurSxDK8kMjTZfprGhh7Yli1SFt8WB3U4zbZ2wxUO7UfprZq3hnl8nSrozO9KDTCDwCYhMgRlcrdwm6XL1uXFwMJt4VSmp1srCQotv0FgT11jF8Km1vMQQOnUC27Va9fbfRtITVsjxsveYeMJqusVWO6F3vAvkM35oL8E8qgBbfrG_lnuhb_9Ws6RIq4YOslkOar_gopEs00CITxmV_aHVHRYzeW7QpycxjC7m8Mp-lKzaUewvJuKWI5HsM134xfaH8RAHSvh6H9pVQAiJ9tyc17bAx46M98WMsHFokVwz3rd7PoGGou6A7y5RzeGpiSxykTWCPPcBnxJ1gwUYqEYGTnRjl9JmhHY_VfQP4edyU-zhmMCCSie8rvkRDilAQGd5kj5m1voSn-EqA13sSe69evXxVUIB2nO70qHCcHBBHxunLqTIIerpc3F9_WWM4_Q_0j9CoTd2aFyuq_sdc6RcmAE3uTznp2DyKNQkT1EfpY7xCCe1MR-Webez5Ioa1EMDP0KrvLdnNRmuM3THSu1pqcvPV7Di7dJci5QWsYEmaP8cLuuZXdAhy_UoSgzbvfT_8mlDoJ9VvDXLJ39OwGYIyZiZ9VTNXm8mxre993cqg7boZRS8x70VRxnjmNxm40SgEvb6CHYO0lSBU"
)

func TestMercure(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	localhost:9080
	
	route {
		mercure {
			anonymous
			publisher_jwt !ChangeMe!
		}

		respond 404
	}`, "caddyfile")

	var connected sync.WaitGroup
	var received sync.WaitGroup
	connected.Add(1)
	received.Add(1)

	go func() {
		cx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequest("GET", "https://localhost:9080/.well-known/mercure?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req = req.WithContext(cx)
		resp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

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
	req, err := http.NewRequest("POST", "https://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode())) //nolint:noctx
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+publisherJWT)

	resp, err := tester.Client.Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	received.Wait()
}

func TestJWTPlaceholders(t *testing.T) {
	k, _ := ioutil.ReadFile("../fixtures/jwt/RS256.key.pub")
	os.Setenv("TEST_JWT_KEY", string(k))
	defer os.Unsetenv("TEST_JWT_KEY")
	os.Setenv("TEST_JWT_ALG", "RS256")
	defer os.Unsetenv("TEST_JWT_ALG")

	tester := caddytest.NewTester(t)
	tester.InitServer(`
	localhost:9080
	
	route {
		mercure {
			anonymous
			publisher_jwt {env.TEST_JWT_KEY} {env.TEST_JWT_ALG}
		}

		respond 404
	}`, "caddyfile")

	var connected sync.WaitGroup
	var received sync.WaitGroup
	connected.Add(1)
	received.Add(1)

	go func() {
		cx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequest("GET", "https://localhost:9080/.well-known/mercure?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req = req.WithContext(cx)
		resp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

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
	req, err := http.NewRequest("POST", "https://localhost:9080/.well-known/mercure", strings.NewReader(body.Encode())) //nolint:noctx
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+publisherJWTRSA)

	resp, err := tester.Client.Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	received.Wait()
}
