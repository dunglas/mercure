package mercure

import (
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
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
	tms := &TopicMatcherStore{}

	l := NewSubscriberList(DefaultSubscriberListCacheSize)
	logger := slog.Default()

	for i := range 100 {
		s := NewLocalSubscriber("", logger, tms)
		t := fmt.Sprintf("https://example.com/%d", i%10)
		s.setMatchers(stringsToExactMatchers([]string{"https://example.org/foo", t}), stringsToExactMatchers([]string{"https://example.net/bar", t}))

		l.Add(s)
	}

	for b.Loop() {
		assert.NotEmpty(b, l.MatchAny(&Update{Topics: []string{"https://example.org/foo"}}))
		assert.Empty(b, l.MatchAny(&Update{Topics: []string{"https://example.org/baz"}}))
		assert.NotEmpty(b, l.MatchAny(&Update{Topics: []string{"https://example.com/8"}, Private: false}))
	}
}

// encode must not reorder the slice it is given: it can be the Update's own
// Topics backing array, so sorting in place would mutate the update being
// dispatched and race with concurrent readers of it.
func TestEncodeDoesNotMutateItsInput(t *testing.T) {
	t.Parallel()

	topics := []string{"https://example.com/z", "https://example.com/a", "https://example.com/m"}
	want := slices.Clone(topics)

	encode(topics, false)

	assert.Equal(t, want, topics)
}

// The cache key must still be one canonical value per topic set, whatever order
// the topics arrive in.
func TestEncodeIsOrderIndependent(t *testing.T) {
	t.Parallel()

	a := encode([]string{"https://example.com/a", "https://example.com/z"}, false)
	b := encode([]string{"https://example.com/z", "https://example.com/a"}, false)

	assert.Equal(t, a, b)
	assert.NotEqual(t, a, encode([]string{"https://example.com/a", "https://example.com/z"}, true))
	assert.NotEqual(t, a, encode([]string{"https://example.com/a"}, false))
}

// A round-trip still recovers the topic set and the private flag, including
// topics containing the escape and delimiter bytes.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	for _, tc := range [][]string{
		{"https://example.com/a"},
		{"https://example.com/z", "https://example.com/a"},
		{"with\x00escape", "with\x01delim"},
	} {
		for _, private := range []bool{false, true} {
			topics, gotPrivate := decode(encode(slices.Clone(tc), private))

			assert.Equal(t, private, gotPrivate)
			assert.ElementsMatch(t, tc, topics)
		}
	}
}
