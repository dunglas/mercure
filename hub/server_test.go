package hub

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const testURL = "http://" + testAddr + "/hub"

func TestServeAllOptions(t *testing.T) {
	h := createAnonymousDummy()
	h.options.Demo = true
	h.options.CorsAllowedOrigins = []string{"*"}

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

	assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-Xss-Protection"))

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

	var wg sync.WaitGroup
	wg.Add(2)

	go func(w *sync.WaitGroup) {
		defer w.Done()
		resp, err := client.Get(testURL + "?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1")
		if err != nil {
			panic(err)
		}

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, []byte("id: first\ndata: hello\n\n"), body)
	}(&wg)

	go func(w *sync.WaitGroup) {
		defer w.Done()
		resp, err := client.Get(testURL + "?topic=http%3A%2F%2Fexample.com%2Falt%2F1")
		if err != nil {
			panic(err)
		}

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, []byte("id: first\ndata: hello\n\n"), body)
	}(&wg)

	// Wait for the subscription
	for {
		if len(h.subscribers) == 2 {
			break
		}
	}

	body := url.Values{"topic": {"http://example.com/foo/1", "http://example.com/alt/1"}, "data": {"hello"}, "id": {"first"}}
	req, _ := http.NewRequest("POST", testURL, strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, true, []string{}))

	_, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	h.server.Shutdown(context.Background())
	wg.Wait()
}
