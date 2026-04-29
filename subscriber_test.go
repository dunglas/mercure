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

// TestBindMatchersRestoresMatcherAfterJSONRoundTrip simulates the path used
// by distributed transports that persist subscribers (the saas Redis
// transport, for instance): a Subscriber is encoded to JSON, decoded back,
// and the receiving end calls BindMatchers to re-resolve the matcher
// implementation that was lost across the unexported field.
func TestBindMatchersRestoresMatcherAfterJSONRoundTrip(t *testing.T) {
	t.Parallel()

	tss := newExactStore(t)
	tss.RegisterMatcherType("URITemplate", URITemplateMatcher)

	src := NewSubscriber(slog.Default(), tss)
	src.setMatchers(
		[]topicMatcher{{Type: "URITemplate", Pattern: "https://example.com/{id}", matcher: URITemplateMatcher}},
		[]topicMatcher{{Type: "Exact", Pattern: "https://example.com/admin", matcher: ExactMatcher}},
	)

	encoded, err := json.Marshal(src)
	require.NoError(t, err)

	dst := NewSubscriber(slog.Default(), tss)
	require.NoError(t, json.Unmarshal(encoded, dst))

	// JSON round-trip drops the unexported matcher binding.
	assert.Nil(t, dst.SubscribedMatchers[0].matcher, "matcher must be lost across JSON round-trip")
	assert.Nil(t, dst.AllowedPrivateMatchers[0].matcher)

	require.NoError(t, dst.BindMatchers())

	assert.Equal(t, URITemplateMatcher, dst.SubscribedMatchers[0].matcher)
	assert.Equal(t, ExactMatcher, dst.AllowedPrivateMatchers[0].matcher)

	// Matching now works again.
	assert.True(t, dst.MatchTopics([]string{"https://example.com/123"}, false))
	assert.True(t, dst.MatchTopics([]string{"https://example.com/admin"}, true))
}

// TestBindMatchersIdempotent verifies that calling BindMatchers a second
// time is a no-op: already-bound matchers keep their existing
// implementation.
func TestBindMatchersIdempotent(t *testing.T) {
	t.Parallel()

	s := NewSubscriber(slog.Default(), newExactStore(t))
	s.setMatchers(
		[]topicMatcher{{Type: "Exact", Pattern: "foo", matcher: ExactMatcher}},
		nil,
	)

	require.NoError(t, s.BindMatchers())
	require.NoError(t, s.BindMatchers())

	assert.Equal(t, ExactMatcher, s.SubscribedMatchers[0].matcher)
}

// TestBindMatchersUnknownType verifies that BindMatchers fails fast when a
// deserialized subscriber references a matcher type that the receiving hub
// does not know about.
func TestBindMatchersUnknownType(t *testing.T) {
	t.Parallel()

	s := NewSubscriber(slog.Default(), newExactStore(t))
	s.SubscribedMatchers = []topicMatcher{{Type: "Bogus", Pattern: "foo"}}

	err := s.BindMatchers()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedMatcherType)
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
