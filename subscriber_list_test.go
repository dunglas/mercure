package mercure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func BenchmarkSubscriberList(b *testing.B) {
	logger := zap.NewNop()

	l := NewSubscriberList(100)
	for i := 0; i < 100; i++ {
		s := NewSubscriber("", logger)
		t := fmt.Sprintf("https://example.com/%d", (i % 10))
		s.SetTopics([]string{"https://example.org/foo", t}, []string{"https://example.net/bar", t})

		l.Add(s)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		assert.NotEmpty(b, l.MatchAny(&Update{Topics: []string{"https://example.org/foo"}}))
		assert.Empty(b, l.MatchAny(&Update{Topics: []string{"https://example.org/baz"}}))
		assert.NotEmpty(b, l.MatchAny(&Update{Topics: []string{"https://example.com/8"}, Private: false}))
	}
}
