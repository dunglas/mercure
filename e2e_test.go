package mercure

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// End-to-end tests that exercise the full wire protocol over a real HTTP server:
// a match_urlpattern SSE subscription authorized by an RFC 9068 at+jwt token
// carrying an RFC 9396 authorization_details claim, private publication, the
// subscription API, and presence through subscription events. This mirrors the
// mechanics of the chat example (examples/chat).

const (
	e2eKey = "e2e-secret"
	e2eAud = "https://hub.example/.well-known/mercure"
	e2eIss = "https://app.example"
)

func e2eToken(t *testing.T, action string, topics []map[string]any, payload any) string {
	t.Helper()

	detail := map[string]any{"type": authorizationDetailTypeMercure, "actions": []string{action}, "topics": topics}
	if payload != nil {
		detail["payload"] = payload
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":                   e2eIss,
		"aud":                   e2eAud,
		"exp":                   4102444800,
		"authorization_details": []any{detail},
	})
	tok.Header["typ"] = "at+jwt"

	s, err := tok.SignedString([]byte(e2eKey))
	require.NoError(t, err)

	return s
}

func e2eHub(t *testing.T) *Hub {
	t.Helper()

	tms, err := NewTopicMatcherStore(DefaultTopicMatcherStoreCacheSize)
	require.NoError(t, err)

	h, err := NewHub(t.Context(),
		WithIssuers([]Issuer{{
			Identifier: e2eIss,
			Publisher:  Static{Key: []byte(e2eKey), Algorithm: "HS256"},
			Subscriber: Static{Key: []byte(e2eKey), Algorithm: "HS256"},
		}}),
		WithResourceIdentifier(e2eAud),
		WithSubscriptions(),
		WithTopicMatcherStore(tms),
		WithTransport(NewLocalTransport(NewSubscriberList(1000))),
	)
	require.NoError(t, err)

	return h
}

// openSSE connects to the subscribe endpoint and streams decoded "data" JSON
// payloads to the returned channel until ctx is done.
func openSSE(ctx context.Context, t *testing.T, base, query, token string) <-chan map[string]any {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/.well-known/mercure?"+query, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed in the streaming goroutine below
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "subscribe status")

	out := make(chan map[string]any, 8)

	go func() {
		defer resp.Body.Close()
		defer close(out)

		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var data strings.Builder

		var event string

		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				event = strings.TrimPrefix(strings.TrimPrefix(line, "event:"), " ")
			case strings.HasPrefix(line, "data:"):
				data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			case line == "":
				if data.Len() == 0 {
					continue
				}

				var m map[string]any
				if json.Unmarshal([]byte(data.String()), &m) == nil {
					// Surface the SSE event name so tests can assert the framing.
					m["__event"] = event

					select {
					case out <- m:
					case <-ctx.Done():
						return
					}
				}

				data.Reset()

				event = ""
			}
		}
	}()

	return out
}

func e2ePublish(t *testing.T, base, token, topic, data string, private bool) {
	t.Helper()

	form := url.Values{"topic": {topic}, "data": {data}}
	if private {
		form.Set("private", "on")
	}

	req, err := http.NewRequest(http.MethodPost, base+"/.well-known/mercure", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "publish status")
	require.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
}

// waitForSubscription polls the subscription API collection URL until it lists
// at least one subscription (confirming the SSE subscriber is registered).
func waitForSubscription(t *testing.T, base, collURL, token string) map[string]any {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, base+collURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, "subscription API status")
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var doc struct {
			Subscriptions []map[string]any `json:"subscriptions"`
		}

		require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))
		resp.Body.Close()

		if len(doc.Subscriptions) > 0 {
			return doc.Subscriptions[0]
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatal("subscription never appeared in the subscription API")

	return nil
}

func TestE2EMessageDeliveryAndSubscriptionAPI(t *testing.T) {
	h := e2eHub(t)

	srv := httptest.NewServer(h)
	defer srv.Close()

	msgPattern := "https://chat.example.com/messages/:id"
	msgTopic := "https://chat.example.com/messages/1"
	collURL := "/.well-known/mercure/subscriptions/urlpattern/" + url.QueryEscape(msgPattern)

	// The token shape the chat example mints: subscribe on the message pattern
	// plus the subscriptions collection (needed to read the subscription API).
	subToken := e2eToken(t, "subscribe",
		[]map[string]any{
			{"match": msgPattern, "match_type": "urlpattern"},
			{"match": collURL, "match_type": "exact"},
		},
		map[string]any{"username": "alice"})
	pubToken := e2eToken(t, "publish",
		[]map[string]any{{"match": msgPattern, "match_type": "urlpattern"}}, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()

	msgs := openSSE(ctx, t, srv.URL, "match_urlpattern="+url.QueryEscape(msgPattern), subToken)

	// Confirm registration via the subscription API (also checks the payload).
	sub := waitForSubscription(t, srv.URL, collURL, subToken)
	require.Equal(t, "urlpattern", sub["match_type"])
	require.Equal(t, msgPattern, sub["match"])
	require.Equal(t, map[string]any{"username": "alice"}, sub["payload"], "payload surfaced in the subscription API")

	// Publish a private message; the subscriber must receive it.
	e2ePublish(t, srv.URL, pubToken, msgTopic, `{"@type":"https://chat.example.com/Message","message":"hi"}`, true)

	select {
	case m := <-msgs:
		require.Equal(t, "hi", m["message"], "received the published message")
	case <-ctx.Done():
		t.Fatal("subscriber did not receive the published message")
	}
}

func TestE2EPresenceViaSubscriptionEvents(t *testing.T) {
	h := e2eHub(t)

	srv := httptest.NewServer(h)
	defer srv.Close()

	msgPattern := "https://chat.example.com/messages/:id"
	enc := url.QueryEscape(msgPattern)
	collURL := "/.well-known/mercure/subscriptions/urlpattern/" + enc
	presencePattern := collURL + "/:subscriber"

	// A watcher subscribes to the presence (subscription-events) topics.
	watcherToken := e2eToken(t, "subscribe",
		[]map[string]any{{"match": presencePattern, "match_type": "urlpattern"}}, nil)

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()

	events := openSSE(ctx, t, srv.URL, "match_urlpattern="+url.QueryEscape(presencePattern), watcherToken)

	time.Sleep(200 * time.Millisecond) // let the watcher register

	// A user joins: subscribing creates a subscription event carrying the payload.
	aToken := e2eToken(t, "subscribe",
		[]map[string]any{{"match": msgPattern, "match_type": "urlpattern"}},
		map[string]any{"username": "alice"})
	_ = openSSE(ctx, t, srv.URL, "match_urlpattern="+enc, aToken)

	select {
	case ev := <-events:
		require.Equal(t, "mercure", ev["__event"], "subscription events carry the reserved SSE event type")
		require.Equal(t, "subscription", ev["type"])
		require.Equal(t, true, ev["active"])
		require.Equal(t, map[string]any{"username": "alice"}, ev["payload"], "presence event carries the payload")
	case <-ctx.Done():
		t.Fatal("watcher did not receive a subscription (presence) event")
	}
}
