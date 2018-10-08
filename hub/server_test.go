package hub

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServe(t *testing.T) {
	h := createAnonymousDummy()

	h.Start()
	go func() {
		h.Serve()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go func(w *sync.WaitGroup) {
		defer w.Done()
		resp, err := http.Get("http://" + testAddr + "/subscribe?topic=http%3A%2F%2Fexample.com%2Ffoo%2F1")
		if err != nil {
			panic(err)
		}

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, []byte("id: first\ndata: hello\n\n"), body)
	}(&wg)

	go func(w *sync.WaitGroup) {
		defer w.Done()
		resp, err := http.Get("http://" + testAddr + "/subscribe?topic=http%3A%2F%2Fexample.com%2Falt%2F1")
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
	req, _ := http.NewRequest("POST", "http://"+testAddr+"/publish", strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, true))

	client := &http.Client{}
	_, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	h.server.Shutdown(context.Background())
	wg.Wait()
}

func TestServeAllOptions(t *testing.T) {
	h := createAnonymousDummy()
	h.options.Demo = true
	h.options.CorsAllowedOrigins = []string{"*"}

	h.Start()
	go func() {
		h.Serve()
	}()

	resp, err := http.Get("http://" + testAddr + "/")
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.NotEmpty(t, body)
	h.server.Shutdown(context.Background())
}
