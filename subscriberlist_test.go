package mercure

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

// Cache misses on concurrent dispatches must not contend on shared state.
func BenchmarkSubscriberListParallelMiss(b *testing.B) {
	tms := &TopicMatcherStore{}
	l := NewSubscriberList(1000)
	logger := slog.Default()

	for i := range 10_000 {
		s := NewLocalSubscriber("", logger, tms)
		s.setMatchers(stringsToExactMatchers([]string{fmt.Sprintf("https://example.com/%d", i)}), nil)

		l.Add(s)
	}

	var n atomic.Int64

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.MatchAny(&Update{Topics: []string{fmt.Sprintf("https://example.com/miss/%d", n.Add(1))}})
		}
	})
}

// The key must not reorder the slice it is given: it can be the Update's own
// Topics backing array, read concurrently.
func TestFilterKeyDoesNotMutateItsInput(t *testing.T) {
	t.Parallel()

	topics := []string{"https://example.com/z", "https://example.com/a", "https://example.com/m"}
	want := slices.Clone(topics)

	newFilterKey(topics, false)

	assert.Equal(t, want, topics)
}

func TestFilterKeyIsOrderIndependent(t *testing.T) {
	t.Parallel()

	a := newFilterKey([]string{"https://example.com/a", "https://example.com/z"}, false)

	assert.Equal(t, a, newFilterKey([]string{"https://example.com/z", "https://example.com/a"}, false))
	assert.NotEqual(t, a, newFilterKey([]string{"https://example.com/a", "https://example.com/z"}, true))
	assert.NotEqual(t, a, newFilterKey([]string{"https://example.com/a"}, false))
}

func TestFilterKeyKeepsTopicBoundaries(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t, newFilterKey([]string{"ab"}, false), newFilterKey([]string{"a", "b"}, false))
	assert.NotEqual(t, newFilterKey([]string{"a", "bc"}, false), newFilterKey([]string{"ab", "c"}, false))
	assert.NotEqual(t, newFilterKey([]string{""}, false), newFilterKey(nil, false))
}

func TestFilterKeyStreaming(t *testing.T) {
	t.Parallel()

	for _, length := range []int{0, 1, 127, 128, 501, 502, 503, 504, 511, 512, 513, 4096, 16384} {
		for _, private := range []bool{false, true} {
			topics := []string{"z", strings.Repeat("a", length), "", "\x00\x01"}
			sorted := slices.Clone(topics)
			slices.Sort(sorted)

			input := []byte{0}
			if private {
				input[0] = 1
			}

			for _, topic := range sorted {
				input = binary.AppendUvarint(input, uint64(len(topic)))
				input = append(input, topic...)
			}

			assert.Equal(t, filterKey(sha256.Sum256(input)), newFilterKey(topics, private), "length=%d private=%t", length, private)
		}
	}
}

func BenchmarkFilterKey(b *testing.B) {
	for _, tc := range []struct {
		name   string
		count  int
		length int
	}{
		{"short", 1, 32},
		{"long", 1, 4096},
		{"large_list", 250, 4000},
	} {
		b.Run(tc.name, func(b *testing.B) {
			topics := make([]string, tc.count)
			for i := range topics {
				topics[i] = fmt.Sprintf("https://example.com/%03d/%s", i, strings.Repeat("a", tc.length))
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				newFilterKey(topics, false)
			}
		})
	}
}

// Filters computed from a digest must still see the update's topics, including
// when a subscriber added later is tested against an already cached filter.
func TestSubscriberListMatchesCachedFilters(t *testing.T) {
	t.Parallel()

	tms := &TopicMatcherStore{}
	logger := slog.Default()
	l := NewSubscriberList(DefaultSubscriberListCacheSize)

	public := NewLocalSubscriber("", logger, tms)
	public.setMatchers(stringsToExactMatchers([]string{"https://example.com/a"}), nil)
	l.Add(public)

	u := &Update{Topics: []string{"https://example.com/a", "https://example.com/b"}, Private: true}
	assert.Empty(t, l.MatchAny(u))
	assert.Equal(t, []*LocalSubscriber{public}, l.MatchAny(&Update{Topics: u.Topics}))

	authorized := NewLocalSubscriber("", logger, tms)
	authorized.setMatchers(stringsToExactMatchers([]string{"https://example.com/b"}), stringsToExactMatchers([]string{"https://example.com/b"}))
	l.Add(authorized)

	assert.Equal(t, []*LocalSubscriber{authorized}, l.MatchAny(u))
}

// Cache keys must not retain the topics of the updates: a publisher sending
// unique, large topic lists would otherwise grow the heap by the size of each
// update, up to the number of cache entries.
func TestSubscriberListCacheDoesNotRetainTopics(t *testing.T) {
	const (
		publishes       = 128
		topicsPerUpdate = 64
	)

	l := NewSubscriberList(DefaultSubscriberListCacheSize)
	padding := strings.Repeat("a", 4096)

	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	for i := range publishes {
		topics := make([]string, topicsPerUpdate)
		for j := range topics {
			topics[j] = fmt.Sprintf("https://example.com/%d/%d/%s", i, j, padding)
		}

		assert.Empty(t, l.MatchAny(&Update{Topics: topics}))
	}

	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(l)

	t.Logf("retained %d bytes after publishing %d MiB of topics", int64(after.HeapAlloc)-int64(before.HeapAlloc), publishes*topicsPerUpdate*len(padding)>>20)
	assert.Less(t, int64(after.HeapAlloc)-int64(before.HeapAlloc), int64(4<<20))
}
