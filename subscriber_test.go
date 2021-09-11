package mercure

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDispatch(t *testing.T) {
	s := NewSubscriber("1", zap.NewNop(), &TopicSelectorStore{})
	s.Topics = []string{"http://example.com"}
	go s.start()
	defer s.Disconnect()

	// Dispatch must be non-blocking
	// Messages coming from the history can be sent after live messages, but must be received first
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "3"}}, false)
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "1"}}, true)
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "4"}}, false)
	s.Dispatch(&Update{Topics: s.Topics, Event: Event{ID: "2"}}, true)
	s.HistoryDispatched("")

	for i := 1; i <= 4; i++ {
		u := <-s.Receive()
		assert.Equal(t, strconv.Itoa(i), u.ID)
	}
}

func TestDisconnect(t *testing.T) {
	s := NewSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.Disconnect()
	// can be called two times without crashing
	s.Disconnect()

	assert.False(t, s.Dispatch(&Update{}, false))
}

func TestLogSubscriber(t *testing.T) {
	sink, logger := newTestLogger(t)
	defer sink.Reset()

	s := NewSubscriber("123", logger, &TopicSelectorStore{})
	s.RemoteAddr = "127.0.0.1"
	s.TopicSelectors = []string{"https://example.com/foo"}
	s.Topics = []string{"https://example.com/bar"}

	f := zap.Object("subscriber", s)
	logger.Info("test", f)

	log := sink.String()
	assert.Contains(t, log, `"last_event_id":"123"`)
	assert.Contains(t, log, `"remote_addr":"127.0.0.1"`)
	assert.Contains(t, log, `"topic_selectors":["https://example.com/foo"]`)
	assert.Contains(t, log, `"topics":["https://example.com/bar"]`)
}

func BenchmarkSubscriber(b *testing.B) {
	for _, concurrency := range []int{
		1,
		10,
		100,
		1000,
		10000,
	} {
		subBenchSubscriber(b, concurrency)
	}
}

func subBenchSubscriber(b *testing.B, concurrency int) {
	tss, err := NewTopicSelectorStore(0, 0)
	if err != nil {
		panic(err)
	}
	b.Run(fmt.Sprintf("concurrency %d", concurrency), func(b *testing.B) {
		var s = NewSubscriber("0e249241-6432-4ce1-b9b9-5d170163c253", zap.NewNop(), tss)
		var wg sync.WaitGroup
		wg.Add(concurrency)
		go s.start()
		defer s.Disconnect()
		ctx, done := context.WithCancel(context.Background())
		defer done()
		go func() {
			for {
				select {
				case <-s.out:
				case <-ctx.Done():
					return
				}
			}
		}()
		b.ResetTimer()
		for i := 0; i < concurrency; i++ {
			go func() {
				for i := 0; i < b.N/concurrency; i++ {
					s.Dispatch(&Update{}, i%2 == 0)
				}
				wg.Done()
			}()
		}
		wg.Wait()
	})
}
