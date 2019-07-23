package hub

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

const testURL = "http://" + testAddr + "/hub"
const testSecureURL = "https://" + testAddr + "/hub"

func TestForwardedHeaders(t *testing.T) {
	h := createAnonymousDummy()
	h.options.Demo = true
	h.options.UseForwardedHeaders = true

	h.Start()
	go func() {
		h.Serve()
	}()

	client := http.Client{Timeout: time.Duration(100 * time.Millisecond)}
	hook := test.NewGlobal()

	// loop until the web server is ready
	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get(testURL)
	}

	body := url.Values{"topic": {"http://example.com/test-forwarded"}, "data": {"hello"}}
	req, _ := http.NewRequest("POST", testURL, strings.NewReader(body.Encode()))
	req.Header.Add("X-Forwarded-For", "192.0.2.1")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, true, []string{}))

	_, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, "192.0.2.1", hook.LastEntry().Data["remote_addr"])

	h.server.Shutdown(context.Background())
}

func TestSecurityOptions(t *testing.T) {
	h := createAnonymousDummy()
	h.options.Demo = true
	h.options.CorsAllowedOrigins = []string{"*"}
	h.options.CertFile = "../fixtures/tls/server.crt"
	h.options.KeyFile = "../fixtures/tls/server.key"
	h.options.Compress = false

	h.Start()
	go func() {
		h.Serve()
	}()

	// This is a self-signed certificate
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transport, Timeout: time.Duration(100 * time.Millisecond)}

	// loop until the web server is ready
	var resp *http.Response
	for resp == nil {
		resp, _ = client.Get(testSecureURL)
	}

	assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-Xss-Protection"))

	// Preflight request
	req, _ := http.NewRequest("OPTIONS", testSecureURL, nil)
	req.Header.Add("Origin", "https://example.com")
	req.Header.Add("Access-Control-Request-Headers", "authorization")
	req.Header.Add("Access-Control-Request-Method", "GET")
	resp, _ = client.Do(req)

	assert.Equal(t, "true", resp.Header.Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Authorization", resp.Header.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	h.server.Shutdown(context.Background())
}

func TestServe(t *testing.T) {
	h := createAnonymousDummy()

	h.Start()
	go func() {
		h.Serve()
	}()

	// loop until the web server is ready
	var resp *http.Response
	client := http.Client{Timeout: time.Duration(100 * time.Millisecond)}
	for resp == nil {
		resp, _ = client.Get("http://" + testAddr + "/")
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
		if err != nil {
			panic(err)
		}
		wgConnected.Done()

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, []byte(":\nid: first\ndata: hello\n\n"), body)
	}()

	go func() {
		defer wgTested.Done()
		resp, err := client.Get(testURL + "?topic=http%3A%2F%2Fexample.com%2Falt%2F1")
		if err != nil {
			panic(err)
		}
		wgConnected.Done()

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, []byte(":\nid: first\ndata: hello\n\n"), body)
	}()

	wgConnected.Wait()

	body := url.Values{"topic": {"http://example.com/foo/1", "http://example.com/alt/1"}, "data": {"hello"}, "id": {"first"}}
	req, _ := http.NewRequest("POST", testURL, strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, true, []string{}))

	_, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	h.server.Shutdown(context.Background())
	wgTested.Wait()
}
