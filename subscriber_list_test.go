package mercure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestEncode(t *testing.T) {
	e := encode([]string{"Foo\x00\x01Bar\x00Baz\x01", "\x01bar"}, true)
	assert.Equal(t, "1\x01\x00\x01bar\x01Foo\x00\x00\x00\x01Bar\x00\x00Baz\x00\x01", e)
}

func TestDecode(t *testing.T) {
	topics, private := decode("1\x01\x00\x01bar\x01Foo\x00\x00\x00\x01Bar\x00\x00Baz\x00\x01")

	assert.Equal(t, []string{"\x01bar", "Foo\x00\x01Bar\x00Baz\x01"}, topics)
	assert.True(t, private)
}

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
