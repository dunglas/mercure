package caddy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/require"
)

const (
	publisherJWT = "eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdfX0.vhMwOaN5K68BTIhWokMLOeOJO4EPfT64brd8euJOA4M"
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
