package mercure

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatch(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	topics := []string{"https://example.com"}
	s := NewLocalSubscriber("1", slog.Default(), &TopicSelectorStore{})
	s.setMatchers(stringsToExactMatchers(topics), stringsToExactMatchers(nil))

	defer s.Disconnect()

	// Dispatch must be non-blocking
	// Messages coming from the history can be sent after live messages, but must be received first
	s.Dispatch(ctx, &Update{Topics: topics, Event: Event{ID: "3"}}, false)
	s.Dispatch(ctx, &Update{Topics: topics, Event: Event{ID: "1"}}, true)
	s.Dispatch(ctx, &Update{Topics: topics, Event: Event{ID: "4"}}, false)
	s.Dispatch(ctx, &Update{Topics: topics, Event: Event{ID: "2"}}, true)
	s.HistoryDispatched("")

	s.Ready(ctx)

	for i := 1; i <= 4; i++ {
		if u, ok := <-s.Receive(); ok && u != nil {
			assert.Equal(t, strconv.Itoa(i), u.ID)
		}
	}
}

func TestDisconnect(t *testing.T) {
	t.Parallel()

	s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
	s.Disconnect()
	// can be called two times without crashing
	s.Disconnect()

	assert.False(t, s.Dispatch(t.Context(), &Update{}, false))
}

func TestLogSubscriber(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	s := NewLocalSubscriber("123", logger, &TopicSelectorStore{})
	s.setMatchers(stringsToExactMatchers([]string{"https://example.com/bar"}), stringsToExactMatchers([]string{"https://example.com/foo"}))

	logger.Info("test", slog.Any("subscriber", s))

	log := buf.String()
	assert.Contains(t, log, `"last_event_id":"123"`)
	assert.Contains(t, log, `"allowed_private_matchers":["Exact:https://example.com/foo"]`)
	assert.Contains(t, log, `"subscribed_matchers":["Exact:https://example.com/bar"]`)
}

// TestSubscriptionPayloadsSurviveJSONRoundTrip simulates the path used by
// distributed transports that persist subscribers (the saas Redis
// transport, for instance): a Subscriber is encoded to JSON and decoded
// back, and the subscription API still renders per-matcher payloads
// without doing any matcher dispatch on the deserialized object.
func TestSubscriptionPayloadsSurviveJSONRoundTrip(t *testing.T) {
	t.Parallel()

	tss := newExactStore(t)
	tss.RegisterMatcherType("URITemplate", URITemplateMatcher)

	src := NewSubscriber(slog.Default(), tss)
	src.Claims = &claims{
		Mercure: mercureClaim{
			Subscribe: []matcherClaim{{
				topicMatcher: topicMatcher{Type: "URITemplate", Pattern: "https://example.com/{id}", matcher: URITemplateMatcher},
				Payload:      map[string]any{"tag": "uritemplate"},
			}},
			Payload: map[string]any{"global": true},
		},
	}
	src.setMatchers(
		[]topicMatcher{
			{Type: "Exact", Pattern: "https://example.com/123", matcher: ExactMatcher},
			{Type: "Exact", Pattern: "https://other.example.com/x", matcher: ExactMatcher},
		},
		nil,
	)

	require.Len(t, src.SubscriptionPayloads, 2)
	assert.Equal(t, map[string]any{"tag": "uritemplate"}, src.SubscriptionPayloads[0], "URITemplate claim accepts /123 → its payload wins")
	assert.Equal(t, map[string]any{"global": true}, src.SubscriptionPayloads[1], "no claim matches /x → fall back to mercure.payload")

	encoded, err := json.Marshal(src)
	require.NoError(t, err)

	dst := NewSubscriber(slog.Default(), tss)
	require.NoError(t, json.Unmarshal(encoded, dst))

	// The matcher implementation is lost across JSON, but the precomputed
	// payloads come back and the subscription API uses them as-is.
	assert.Nil(t, dst.SubscribedMatchers[0].matcher, "matcher field is unexported and cannot survive JSON")
	require.Len(t, dst.SubscriptionPayloads, 2)

	subs := dst.getSubscriptions(subscriptionFilter{}, "", true)
	require.Len(t, subs, 2)
	assert.Equal(t, map[string]any{"tag": "uritemplate"}, subs[0].Payload)
	assert.Equal(t, map[string]any{"global": true}, subs[1].Payload)
}

func TestSubscriberDoesNotBlockWhenChanIsFull(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
	s.Ready(ctx)

	for i := 0; i <= outBufferLength; i++ {
		s.Dispatch(ctx, &Update{}, false)
	}

	for range s.Receive() { //nolint:revive
	}
}
