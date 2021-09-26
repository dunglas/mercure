package mercure

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDispatch(t *testing.T) {
	s := NewSubscriber("1", zap.NewNop())
	s.Topics = []string{"http://example.com"}
	defer s.Disconnect()

	// Dispatch must be non-blocking
	// Messages coming from the history can be sent after live messages, but must be received first
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "3"}}, false)
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "1"}}, true)
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "4"}}, false)
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "2"}}, true)
	s.HistoryDispatched("")

	s.Ready()

	for i := 1; i <= 4; i++ {
		if u, ok := <-s.Receive(); ok && u != nil {
			assert.Equal(t, strconv.Itoa(i), u.ID)
		}
	}
}

func TestDisconnect(t *testing.T) {
	s := NewSubscriber("", zap.NewNop())
	s.Disconnect()
	// can be called two times without crashing
	s.Disconnect()

	assert.False(t, s.Dispatch(&Update{}, false))
}

func TestLogSubscriber(t *testing.T) {
	sink, logger := newTestLogger(t)
	defer sink.Reset()

	s := NewSubscriber("123", logger)
	s.RemoteAddr = "127.0.0.1"
	s.SetTopics([]string{"https://example.com/bar"}, []string{"https://example.com/foo"})

	f := zap.Object("subscriber", s)
	logger.Info("test", f)

	log := sink.String()
	assert.Contains(t, log, `"last_event_id":"123"`)
	assert.Contains(t, log, `"remote_addr":"127.0.0.1"`)
	assert.Contains(t, log, `"topic_selectors":["https://example.com/foo"]`)
	assert.Contains(t, log, `"topics":["https://example.com/bar"]`)
}
