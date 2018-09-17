package hub

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServe(t *testing.T) {
	h := createAnonymousDummy()
	h.Start()
	go func() {
		h.Serve()
	}()
	// Wait for the HTTP server to start...
	time.Sleep(1 * time.Second)

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
	time.Sleep(1 * time.Second)

	body := url.Values{"topic": {"http://example.com/foo/1", "http://example.com/alt/1"}, "data": {"hello"}, "id": {"first"}}
	req, _ := http.NewRequest("POST", "http://"+testAddr+"/publish", strings.NewReader(body.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+createDummyAuthorizedJWT(h, true))

	client := &http.Client{}
	_, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	h.Stop()
	wg.Wait()
}
