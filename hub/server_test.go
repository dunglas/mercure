package hub

import (
	"context"
	"crypto/tls"
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
	h := createDummyWithTransportAndConfig(NewLocalTransport(), v)

	go func() {
		h.Serve()
	}()

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
	h := createDummyWithTransportAndConfig(NewLocalTransport(), v)

	go func() {
		h.Serve()
	}()

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

	hpBody, _ := ioutil.ReadAll(resp.Body)

	assert.Contains(t, string(hpBody), "Mercure Hub")

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

func TestServeAcme(t *testing.T) {
	dir, _ := ioutil.TempDir("", "cert")
	defer os.RemoveAll(dir)

	v := viper.New()
	v.Set("acme_hosts", []string{"example.com"})
	v.Set("acme_http01_addr", ":8080")
	v.Set("acme_cert_dir", dir)
	h := createDummyWithTransportAndConfig(NewLocalTransport(), v)

	go func() {
		h.Serve()
	}()

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
