package mercure

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkLocalTransport(b *testing.B) {
	subscribeBenchmarkHelper(b, subBenchLocalTransport)
}

func subBenchLocalTransport(b *testing.B, topics, concurrency, matchPct int, testName string) {
	b.Helper()

	tr := NewLocalTransport(NewSubscriberList(1_000))

	b.Cleanup(func() {
		assert.NoError(b, tr.Close())
	})

	top := make([]string, topics)
	tsMatch := make([]string, topics)

	tsNoMatch := make([]string, topics)
	for i := range topics {
		tsNoMatch[i] = fmt.Sprintf("/%d/{%d}", rand.Int(), rand.Int()) //nolint:gosec
		if topics/2 == i {
			n1 := rand.Int() //nolint:gosec
			n2 := rand.Int() //nolint:gosec
			top[i] = fmt.Sprintf("/%d/%d", n1, n2)
			tsMatch[i] = fmt.Sprintf("/%d/{%d}", n1, n2)
		} else {
			top[i] = fmt.Sprintf("/%d/%d", rand.Int(), rand.Int()) //nolint:gosec
			tsMatch[i] = tsNoMatch[i]
		}
	}

	tss := &TopicSelectorStore{}

	subscribers := make([]*LocalSubscriber, concurrency)
	for i := range concurrency {
		s := NewLocalSubscriber("", slog.Default(), tss)
		if i%100 < matchPct {
			s.SetTopics(tsMatch, nil)
		} else {
			s.SetTopics(tsNoMatch, nil)
		}

		subscribers[i] = s
		require.NoError(b, tr.AddSubscriber(s))
	}

	ctx, done := context.WithCancel(b.Context())
	b.Cleanup(done)

	for i := range concurrency {
		go func() {
			for {
				select {
				case _, ok := <-subscribers[i].Receive():
					if !ok {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	b.SetParallelism(concurrency)
	b.Run(testName, func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for i := 0; pb.Next(); i++ {
				require.NoError(b, tr.Dispatch(&Update{Topics: top}))
			}
		})
	})
}

/*
These are example commands that can be used to run subsets of this test for analysis.
Omission of any environment variable causes the test to enumerate a few meaningful options.

SUB_TEST_CONCURRENCY=20000 \
	SUB_TEST_TOPICS=20 \
	SUB_TEST_MATCHPCT=50 \
	go test -bench=. -run=BenchmarkLocalTransport -cpuprofile profile.out -benchmem
go tool pprof --pdf _dist/bin profile.out > profile.pdf
*/
