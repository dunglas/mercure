package mercure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestEncode(t *testing.T) {
	t.Parallel()

	e := encode([]string{"Foo\x00\x01Bar\x00Baz\x01", "\x01bar"}, true)
	assert.Equal(t, "1\x01\x00\x01bar\x01Foo\x00\x00\x00\x01Bar\x00\x00Baz\x00\x01", e)
}

func TestDecode(t *testing.T) {
	t.Parallel()

	topics, private := decode("1\x01\x00\x01bar\x01Foo\x00\x00\x00\x01Bar\x00\x00Baz\x00\x01")

	assert.Equal(t, []string{"\x01bar", "Foo\x00\x01Bar\x00Baz\x01"}, topics)
	assert.True(t, private)
}

func BenchmarkSubscriberList(b *testing.B) {
	logger := zap.NewNop()
	tss := &TopicSelectorStore{}

	l := NewSubscriberList(100)

	for i := range 100 {
		s := NewLocalSubscriber("", logger, tss)
		t := fmt.Sprintf("https://example.com/%d", i%10)
		s.SetTopics([]string{"https://example.org/foo", t}, []string{"https://example.net/bar", t})

		l.Add(s)
	}

	for b.Loop() {
		assert.NotEmpty(b, l.MatchAny(&Update{Topics: []string{"https://example.org/foo"}}))
		assert.Empty(b, l.MatchAny(&Update{Topics: []string{"https://example.org/baz"}}))
		assert.NotEmpty(b, l.MatchAny(&Update{Topics: []string{"https://example.com/8"}, Private: false}))
	}
}
