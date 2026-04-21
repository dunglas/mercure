package mercure

import (
	"bytes"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
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
