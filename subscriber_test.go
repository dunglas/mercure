package mercure

import (
	"bytes"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatch(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	topic := "https://example.com"
	s := NewLocalSubscriber("1", slog.Default(), &TopicSelectorStore{})
	s.setMatchers(stringsToExactMatchers([]string{topic}), nil)

	defer s.Disconnect()

	// Dispatch must be non-blocking
	// Messages coming from the history can be sent after live messages, but must be received first
	s.Dispatch(ctx, &Update{Topic: topic, Event: Event{ID: "3"}}, false)
	s.Dispatch(ctx, &Update{Topic: topic, Event: Event{ID: "1"}}, true)
	s.Dispatch(ctx, &Update{Topic: topic, Event: Event{ID: "4"}}, false)
	s.Dispatch(ctx, &Update{Topic: topic, Event: Event{ID: "2"}}, true)
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
	assert.Contains(t, log, `"allowed_private_matchers":["exact:https://example.com/foo"]`)
	assert.Contains(t, log, `"subscribed_matchers":["exact:https://example.com/bar"]`)
}

func TestMatchTopic(t *testing.T) {
	t.Parallel()

	tss, err := NewTopicSelectorStore(0)
	require.NoError(t, err)

	s := NewLocalSubscriber("", slog.Default(), tss)
	s.setMatchers([]TopicMatcher{
		{Type: MatcherTypeExact, Pattern: "https://example.com/no-match"},
		{Type: MatcherTypeURLPattern, Pattern: "https://example.com/books/:id"},
	}, []TopicMatcher{
		{Type: MatcherTypeURLPattern, Pattern: "https://example.com/users/foo/*"},
	})

	assert.False(t, s.Match(&Update{Topic: "https://example.com/not-subscribed"}))
	assert.False(t, s.Match(&Update{Topic: "https://example.com/not-subscribed", Private: true}))
	assert.False(t, s.Match(&Update{Topic: "https://example.com/no-match", Private: true}))
	assert.False(t, s.Match(&Update{Topic: "https://example.com/books/1", Private: true}))

	assert.True(t, s.Match(&Update{Topic: "https://example.com/books/1"}))
	assert.True(t, s.Match(&Update{Topic: "https://example.com/no-match"}))
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
