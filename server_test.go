package mercure

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const (
	testURLscheme = "http://"
	testURL       = testURLscheme + testAddr + defaultHubURL

	testSecureURLScheme = "https://"
	testSecureURL       = testSecureURLScheme + testAddr + defaultHubURL
)

func TestForwardedHeaders(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	h := createDummy(WithLogger(zap.New(core)))
	h.config.Set("use_forwarded_headers", true)

	go h.Serve()

	client := http.Client{Timeout: 100 * time.Millisecond}

	// loop until the web server is ready
	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get(testURL) //nolint:bodyclose
	}
	defer resp.Body.Close()

	body := url.Values{"topic": {"http://example.com/test-forwarded"}, "data": {"hello"}}
	req, _ := http.NewRequest(http.MethodPost, testURL, strings.NewReader(body.Encode()))
	req.Header.Add("X-Forwarded-For", "192.0.2.1")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	resp2, err := client.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, 1, logs.FilterField(zap.String("remote_addr", "192.0.2.1")).Len())

	h.server.Shutdown(context.Background())
}

func TestSecurityOptions(t *testing.T) {
	h := createAnonymousDummy(WithSubscriptions(), WithDemo(), WithCORSOrigins([]string{"*"}))
	h.config.Set("cert_file", "fixtures/tls/server.crt")
	h.config.Set("key_file", "fixtures/tls/server.key")
	h.config.Set("compress", true)

	go h.Serve()

	// This is a self-signed certificate
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	client := http.Client{Transport: transport, Timeout: 100 * time.Millisecond}

	// loop until the web server is ready
	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get(testSecureURL) //nolint:bodyclose
	}

	assert.Equal(t, "default-src 'self' mercure.rocks cdn.jsdelivr.net", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-Xss-Protection"))
	resp.Body.Close()

	// Preflight request
	req, _ := http.NewRequest(http.MethodOptions, testSecureURL, nil)
	req.Header.Add("Origin", "https://example.com")
	req.Header.Add("Access-Control-Request-Headers", "authorization,cache-control,last-event-id")
	req.Header.Add("Access-Control-Request-Method", http.MethodGet)
	resp2, _ := client.Do(req)
	require.NotNil(t, resp2)

	assert.Equal(t, "true", resp2.Header.Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Authorization,Cache-Control,Last-Event-Id", resp2.Header.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "*", resp2.Header.Get("Access-Control-Allow-Origin"))
	resp2.Body.Close()

	// Subscriptions
	req, _ = http.NewRequest(http.MethodGet, testSecureURL+subscriptionsPath, nil)
	resp3, _ := client.Do(req)
	require.NotNil(t, resp3)
	assert.Equal(t, http.StatusUnauthorized, resp3.StatusCode)
	resp3.Body.Close()

	h.server.Shutdown(context.Background())
}

func TestSecurityOptionsWithCorsOrigin(t *testing.T) {
	h := createDummy(WithSubscriptions(), WithCORSOrigins([]string{"https://subscriber.com"}))
	h.config.Set("cert_file", "fixtures/tls/server.crt")
	h.config.Set("key_file", "fixtures/tls/server.key")
	h.config.Set("compress", true)

	go h.Serve()

	// This is a self-signed certificate
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	client := http.Client{Transport: transport, Timeout: 100 * time.Millisecond}

	// loop until the web server is ready
	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get(testSecureURL) //nolint:bodyclose
	}

	assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-Xss-Protection"))
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodOptions, testSecureURL, nil)

	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(roleSubscriber, []string{}))
	req.Header.Add("Content-Type", "text/plain; boundary=")
	req.Header.Add("Origin", "https://subscriber.com")
	req.Header.Add("Host", "subscriber.com")
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Pragma", "no-cache")
	req.Header.Add("Access-Control-Request-Headers", "authorization,cache-control,last-event-id")
	req.Header.Add("Access-Control-Request-Method", http.MethodGet)
	resp2, _ := client.Do(req)
	require.NotNil(t, resp2)

	assert.Equal(t, "true", resp2.Header.Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Authorization,Cache-Control,Last-Event-Id", resp2.Header.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "https://subscriber.com", resp2.Header.Get("Access-Control-Allow-Origin"))
	resp2.Body.Close()

	h.server.Shutdown(context.Background())
}

func TestServe(t *testing.T) {
	h := createAnonymousDummy()

	go h.Serve()

	// loop until the web server is ready
	var resp *http.Response
	client := http.Client{Timeout: 100 * time.Millisecond}
	for resp == nil {
		resp, _ = client.Get(testURLscheme + testAddr + "/") //nolint:bodyclose
	}
	defer resp.Body.Close()

	hpBody, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(hpBody), "Mercure Hub")

	respHealthz, err := client.Get(testURLscheme + testAddr + "/healthz")
	require.NoError(t, err)
	defer respHealthz.Body.Close()
	healthzBody, _ := io.ReadAll(respHealthz.Body)
	assert.Contains(t, string(healthzBody), "ok")

	var wgConnected, wgTested sync.WaitGroup
	wgConnected.Add(2)
	wgTested.Add(2)

	go func() {
		defer wgTested.Done()
		resp, err := client.Get(testURL + "?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1")
		assert.NoError(t, err)
		wgConnected.Done()

		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, []byte(":\nid: first\ndata: hello\n\n"), body)
	}()

	go func() {
		defer wgTested.Done()
		resp, err := client.Get(testURL + "?topic=http%3A%2F%2Fexample.com%2Falt%2F1")
		assert.NoError(t, err)
		wgConnected.Done()

		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, []byte(":\nid: first\ndata: hello\n\n"), body)
	}()

	wgConnected.Wait()

	body := url.Values{"topic": {"http://example.com/foo/1", "http://example.com/alt/1"}, "data": {"hello"}, "id": {"first"}}
	req, _ := http.NewRequest(http.MethodPost, testURL, strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	resp2, err := client.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	h.server.Shutdown(context.Background())
	wgTested.Wait()
}

func TestClientClosesThenReconnects(t *testing.T) {
	l := zap.NewNop()
	u, err := url.Parse("bolt://test.db")
	require.NoError(t, err)

	bt, err := NewTransport(u, l)
	require.NoError(t, err)
	defer os.Remove("test.db")

	h := createAnonymousDummy(WithLogger(l), WithTransport(bt))
	transport := h.transport.(*BoltTransport)
	go h.Serve()

	// loop until the web server is ready
	var resp *http.Response
	client := http.Client{Timeout: 10 * time.Second}
	for resp == nil {
		resp, _ = client.Get(testURLscheme + testAddr) //nolint:bodyclose
	}
	require.NoError(t, resp.Body.Close())

	var wg sync.WaitGroup

	subscribe := func(expectedBodyData string) {
		cx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequest(http.MethodGet, testURL+"?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req = req.WithContext(cx)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		var receivedBody strings.Builder
		buf := make([]byte, 1024)
		for {
			_, err := resp.Body.Read(buf)
			if errors.Is(err, io.EOF) {
				panic("EOF")
			}

			receivedBody.Write(buf)
			if strings.Contains(receivedBody.String(), "data: "+expectedBodyData+"\n") {
				cancel()

				break
			}
		}

		require.NoError(t, resp.Body.Close())
		wg.Done()
	}

	publish := func(data string, waitForSubscribers int) {
		for {
			transport.RLock()
			l := transport.subscribers.Len()
			transport.RUnlock()
			if l >= waitForSubscribers {
				break
			}
		}

		body := url.Values{"topic": {"http://example.com/foo/1"}, "data": {data}, "id": {data}}
		req, err := http.NewRequest(http.MethodPost, testURL, strings.NewReader(body.Encode()))
		require.NoError(t, err)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())

		wg.Done()
	}

	nbSubscribers := 10
	wg.Add(nbSubscribers + 1)
	for i := 0; i < nbSubscribers; i++ {
		go subscribe("first")
	}

	publish("first", nbSubscribers)
	wg.Wait()

	nbPublishers := 5
	wg.Add(nbPublishers)
	for i := 0; i < nbPublishers; i++ {
		go publish("lost", 0)
	}
	wg.Wait()

	nbSubscribers = 20
	nbPublishers = 10
	wg.Add(nbSubscribers + nbPublishers)
	for i := 0; i < nbSubscribers; i++ {
		go subscribe("second")
	}
	for i := 0; i < nbPublishers; i++ {
		go publish("second", nbSubscribers)
	}
	wg.Wait()
	h.server.Shutdown(context.Background())
}

func TestServeAcme(t *testing.T) {
	dir, _ := os.MkdirTemp("", "cert")
	defer os.RemoveAll(dir)

	h := createAnonymousDummy(WithAllowedHosts([]string{"example.com"}))
	h.config.Set("acme_http01_addr", ":8080")
	h.config.Set("acme_http01_addr", ":8080")
	h.config.Set("acme_cert_dir", dir)

	go h.Serve()
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get("http://127.0.0.1:8080") //nolint:bodyclose
	}

	require.NotNil(t, resp)
	assert.Equal(t, 302, resp.StatusCode)
	resp.Body.Close()

	resp, err := client.Get("http://0.0.0.0:8080/.well-known/acme-challenge/does-not-exists")
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, 403, resp.StatusCode)
	h.server.Shutdown(context.Background())
}

func TestMetricsAccess(t *testing.T) {
	server := newTestServer(t)
	defer server.shutdown()

	resp, err := server.client.Get(testURLscheme + testMetricsAddr + metricsPath)
	require.NoError(t, err)
	defer resp.Body.Close()

	resp, err = server.client.Get(testURLscheme + testMetricsAddr + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}

func TestMetricsCollect(t *testing.T) {
	server := newTestServer(t)
	defer server.shutdown()

	server.newSubscriber("http://example.com/foo/1", true)
	server.newSubscriber("http://example.com/alt/1", true)
	server.newSubscriber("http://example.com/alt/1", true)
	server.newSubscriber("http://example.com/alt/1", false)
	server.waitSubscribers()

	body := url.Values{"topic": {"http://example.com/foo/1", "http://example.com/alt/1"}, "data": {"hello"}, "id": {"first"}}
	server.publish(body)

	body = url.Values{"topic": {"http://example.com/foo/1"}, "data": {"second hello"}, "id": {"second"}}
	server.publish(body)

	server.assertMetric("mercure_subscribers_connected 3")
	server.assertMetric("mercure_subscribers_total 4")
	server.assertMetric("mercure_updates_total 2")
}

func TestMetricsVersionIsAccessible(t *testing.T) {
	server := newTestServer(t)
	defer server.shutdown()

	resp, err := server.client.Get(testURLscheme + testMetricsAddr + metricsPath)
	require.NoError(t, err)
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	pattern := "mercure_version_info{architecture=\".+\",built_at=\".*\",commit=\".*\",go_version=\".+\",os=\".+\",version=\"dev\"} 1"
	assert.Regexp(t, regexp.MustCompile(pattern), string(b))
	server.assertMetric("mercure_version_info")
}

type testServer struct {
	h           *Hub
	client      http.Client
	t           *testing.T
	wgShutdown  *sync.WaitGroup
	wgConnected sync.WaitGroup
	wgTested    sync.WaitGroup
}

func newTestServer(t *testing.T) testServer {
	t.Helper()

	m := NewPrometheusMetrics(nil)
	h := createAnonymousDummy(WithMetrics(m))

	go h.Serve()

	// loop until the web server is ready
	var resp *http.Response
	client := http.Client{Timeout: 100 * time.Millisecond}
	for resp == nil {
		resp, _ = client.Get(testURLscheme + testAddr + "/") //nolint:bodyclose
	}
	defer resp.Body.Close()

	var wgShutdown sync.WaitGroup
	wgShutdown.Add(1)

	return testServer{
		h,
		client,
		t,
		&wgShutdown,
		sync.WaitGroup{},
		sync.WaitGroup{},
	}
}

func (s *testServer) shutdown() {
	s.h.server.Shutdown(context.Background())
	s.h.metricsServer.Shutdown(context.Background())
	s.wgShutdown.Done()
	s.wgTested.Wait()
}

func (s *testServer) newSubscriber(topic string, keepAlive bool) {
	s.wgConnected.Add(1)
	s.wgTested.Add(1)

	go func() {
		defer s.wgTested.Done()
		resp, err := s.client.Get(testURL + "?topic=" + url.QueryEscape(topic))
		require.NoError(s.t, err)
		defer resp.Body.Close()
		s.wgConnected.Done()

		if keepAlive {
			s.wgShutdown.Wait()
		}
	}()
}

func (s *testServer) publish(body url.Values) {
	req, _ := http.NewRequest(http.MethodPost, testURL, strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	resp, err := s.client.Do(req)
	require.NoError(s.t, err)
	defer resp.Body.Close()
}

func (s *testServer) waitSubscribers() {
	s.wgConnected.Wait()
}

func (s *testServer) assertMetric(metric string) {
	resp, err := s.client.Get(testURLscheme + testMetricsAddr + metricsPath)
	require.NoError(s.t, err)
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	require.NoError(s.t, err)

	assert.Contains(s.t, string(b), metric)
}
