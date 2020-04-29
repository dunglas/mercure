package hub

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testURL = "http://" + testAddr + defaultHubURL
const testSecureURL = "https://" + testAddr + defaultHubURL

func TestForwardedHeaders(t *testing.T) {
	v := viper.New()
	v.Set("use_forwarded_headers", true)
	h := createDummyWithTransportAndConfig(NewLocalTransport(5, time.Second), v)

	go h.Serve()

	client := http.Client{Timeout: 100 * time.Millisecond}
	hook := test.NewGlobal()

	// loop until the web server is ready
	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get(testURL) //nolint:bodyclose
	}
	defer resp.Body.Close()

	body := url.Values{"topic": {"http://example.com/test-forwarded"}, "data": {"hello"}}
	req, _ := http.NewRequest("POST", testURL, strings.NewReader(body.Encode()))
	req.Header.Add("X-Forwarded-For", "192.0.2.1")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, publisherRole, []string{}))

	resp2, err := client.Do(req)
	require.Nil(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, "192.0.2.1", hook.LastEntry().Data["remote_addr"])

	h.server.Shutdown(context.Background())
}

func TestSecurityOptions(t *testing.T) {
	v := viper.New()
	v.Set("demo", true)
	v.Set("cors_allowed_origins", []string{"*"})
	v.Set("cert_file", "../fixtures/tls/server.crt")
	v.Set("key_file", "../fixtures/tls/server.key")
	v.Set("compress", true)
	h := createDummyWithTransportAndConfig(NewLocalTransport(5, time.Second), v)

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
	defer resp.Body.Close()

	assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-Xss-Protection"))

	// Preflight request
	req, _ := http.NewRequest("OPTIONS", testSecureURL, nil)
	req.Header.Add("Origin", "https://example.com")
	req.Header.Add("Access-Control-Request-Headers", "authorization")
	req.Header.Add("Access-Control-Request-Method", "GET")
	resp2, _ := client.Do(req)
	require.NotNil(t, resp2)
	defer resp2.Body.Close()

	assert.Equal(t, "true", resp2.Header.Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Authorization", resp2.Header.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "*", resp2.Header.Get("Access-Control-Allow-Origin"))

	h.server.Shutdown(context.Background())
}

func TestServe(t *testing.T) {
	h := createAnonymousDummy()

	go h.Serve()

	// loop until the web server is ready
	var resp *http.Response
	client := http.Client{Timeout: 100 * time.Millisecond}
	for resp == nil {
		resp, _ = client.Get("http://" + testAddr + "/") //nolint:bodyclose
	}
	defer resp.Body.Close()

	hpBody, _ := ioutil.ReadAll(resp.Body)
	assert.Contains(t, string(hpBody), "Mercure Hub")

	respHealthz, err := client.Get("http://" + testAddr + "/healthz")
	require.Nil(t, err)
	defer respHealthz.Body.Close()
	healthzBody, _ := ioutil.ReadAll(respHealthz.Body)
	assert.Contains(t, string(healthzBody), "ok")

	respMetrics, err := client.Get("http://" + testAddr + "/metrics")
	require.Nil(t, err)
	defer respMetrics.Body.Close()
	assert.Equal(t, 404, respMetrics.StatusCode)

	var wgConnected, wgTested sync.WaitGroup
	wgConnected.Add(2)
	wgTested.Add(2)

	go func() {
		defer wgTested.Done()
		resp, err := client.Get(testURL + "?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1")
		require.Nil(t, err)
		wgConnected.Done()

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, []byte(":\nid: first\ndata: hello\n\n"), body)
	}()

	go func() {
		defer wgTested.Done()
		resp, err := client.Get(testURL + "?topic=http%3A%2F%2Fexample.com%2Falt%2F1")
		require.Nil(t, err)
		wgConnected.Done()

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, []byte(":\nid: first\ndata: hello\n\n"), body)
	}()

	wgConnected.Wait()

	body := url.Values{"topic": {"http://example.com/foo/1", "http://example.com/alt/1"}, "data": {"hello"}, "id": {"first"}}
	req, _ := http.NewRequest("POST", testURL, strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, publisherRole, []string{}))

	resp2, err := client.Do(req)
	require.Nil(t, err)
	defer resp2.Body.Close()

	h.server.Shutdown(context.Background())
	wgTested.Wait()
}

func TestClientClosesThenReconnects(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u, 5, time.Second)
	defer os.Remove("test.db")

	h := createDummyWithTransportAndConfig(transport, viper.New())
	go h.Serve()

	// loop until the web server is ready
	var resp *http.Response
	client := http.Client{Timeout: 10 * time.Second}
	for resp == nil {
		resp, _ = client.Get("http://" + testAddr + "/") //nolint:bodyclose
	}
	resp.Body.Close()

	var wg sync.WaitGroup

	subscribe := func(expectedBodyData string) {
		cx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequest("GET", testURL+"?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1", nil)
		req = req.WithContext(cx)
		resp, err := http.DefaultClient.Do(req)
		require.Nil(t, err)

		receivedBody := ""
		buf := make([]byte, 1)
		for {
			_, err := resp.Body.Read(buf)
			if err == io.EOF {
				panic("EOF")
			}

			receivedBody += string(buf)
			if strings.Contains(receivedBody, "data: "+expectedBodyData+"\n") {
				cancel()
				break
			}
		}

		resp.Body.Close()
		wg.Done()
	}

	publish := func(data string, waitForSubscribers int) {
		for {
			transport.Lock()
			l := len(transport.pipes)
			transport.Unlock()
			if l >= waitForSubscribers {
				break
			}
		}

		body := url.Values{"topic": {"http://example.com/foo/1"}, "data": {data}, "id": {data}}
		req, err := http.NewRequest("POST", testURL, strings.NewReader(body.Encode()))
		require.Nil(t, err)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, publisherRole, []string{}))

		resp, err := client.Do(req)
		require.Nil(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

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
	dir, _ := ioutil.TempDir("", "cert")
	defer os.RemoveAll(dir)

	v := viper.New()
	v.Set("acme_hosts", []string{"example.com"})
	v.Set("acme_http01_addr", ":8080")
	v.Set("acme_cert_dir", dir)
	h := createDummyWithTransportAndConfig(NewLocalTransport(5, time.Second), v)

	go h.Serve()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
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
	assert.Nil(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, 403, resp.StatusCode)

	h.server.Shutdown(context.Background())
}

func TestMetricsAccess(t *testing.T) {
	v := viper.New()
	v.Set("metrics", true)
	h := createDummyWithTransportAndConfig(NewLocalTransport(5, time.Second), v)

	go h.Serve()

	client := http.Client{Timeout: 100 * time.Millisecond}

	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get("http://" + testAddr + "/metrics") //nolint:bodyclose
	}
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	h.server.Shutdown(context.Background())
}

func TestMetricsCollect(t *testing.T) {
	v := viper.New()
	v.Set("metrics", true)
	server := newTestServer(t, v)

	server.newSubscriber("http://example.com/foo/1", true)
	server.newSubscriber("http://example.com/alt/1", true)
	server.newSubscriber("http://example.com/alt/1", true)
	server.newSubscriber("http://example.com/alt/1", false)
	server.waitSubscribers()

	body := url.Values{"topic": {"http://example.com/foo/1", "http://example.com/alt/1"}, "data": {"hello"}, "id": {"first"}}
	server.publish(body)

	body = url.Values{"topic": {"http://example.com/foo/1"}, "data": {"second hello"}, "id": {"second"}}
	server.publish(body)

	server.assertMetric("mercure_subcribers{topic=\"http://example.com/foo/1\"} 1")
	server.assertMetric("mercure_subcribers{topic=\"http://example.com/alt/1\"} 2")
	server.assertMetric("mercure_subcribers_total{topic=\"http://example.com/foo/1\"} 1")
	server.assertMetric("mercure_subcribers_total{topic=\"http://example.com/alt/1\"} 3")
	server.assertMetric("mercure_updates_total{topic=\"http://example.com/foo/1\"} 2")
	server.assertMetric("mercure_updates_total{topic=\"http://example.com/alt/1\"} 1")

	server.shutdown()
}

type testServer struct {
	h           *Hub
	client      http.Client
	t           *testing.T
	wgShutdown  *sync.WaitGroup
	wgConnected sync.WaitGroup
	wgTested    sync.WaitGroup
}

func newTestServer(t *testing.T, v *viper.Viper) testServer {
	h := createDummyWithTransportAndConfig(NewLocalTransport(5, time.Second), v)

	go func() {
		h.Serve()
	}()

	// loop until the web server is ready
	var resp *http.Response
	client := http.Client{Timeout: 100 * time.Millisecond}
	for resp == nil {
		resp, _ = client.Get("http://" + testAddr + "/") //nolint:bodyclose
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
	s.wgShutdown.Done()
	s.wgTested.Wait()
}

func (s *testServer) newSubscriber(topic string, keepAlive bool) {
	s.wgConnected.Add(1)
	s.wgTested.Add(1)

	go func() {
		defer s.wgTested.Done()
		resp, err := s.client.Get(testURL + "?topic=" + url.QueryEscape(topic))
		require.Nil(s.t, err)
		defer resp.Body.Close()
		s.wgConnected.Done()

		if keepAlive {
			s.wgShutdown.Wait()
		}
	}()
}

func (s *testServer) publish(body url.Values) {
	req, _ := http.NewRequest("POST", testURL, strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(s.h, publisherRole, []string{}))

	resp, err := s.client.Do(req)
	require.Nil(s.t, err)
	defer resp.Body.Close()
}

func (s *testServer) waitSubscribers() {
	s.wgConnected.Wait()
}

func (s *testServer) assertMetric(metric string) {
	resp, err := s.client.Get("http://" + testAddr + "/metrics")
	assert.Nil(s.t, err)
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	assert.Nil(s.t, err)

	assert.Contains(s.t, string(b), metric)
}
