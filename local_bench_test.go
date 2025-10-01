package mercure

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func BenchmarkLocalTransport(b *testing.B) {
	subscribeBenchmarkHelper(b, subBenchLocalTransport)
}

func subBenchLocalTransport(b *testing.B, topics, concurrency, matchPct int, testName string) {
	b.Helper()

	tr := NewLocalTransport()

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
	logger := zap.NewNop()

	subscribers := make([]*LocalSubscriber, concurrency)
	for i := range concurrency {
		s := NewLocalSubscriber("", logger, tss)
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

/* --- test.sh ---
These are example commands that can be used to run subsets of this test for analysis.
Omission of any environment variable causes the test to enumerate a few meaningful options.

#!/usr/bin/sh

set -e

mkdir -p _dist

# --- Generating a cpu call graph ---

SUB_TEST_CONCURRENCY=20000 \
SUB_TEST_TOPICS=20 \
SUB_TEST_MATCHPCT=50 \
SUB_TEST_SHARDS=256 \
go test -bench=. -run=BenchmarkLocalTransport -cpuprofile _dist/profile.20kc.20top.50pct.noskip.256sh.out -benchmem

go build -o _dist/bin

go tool pprof --pdf _dist/bin _dist/profile.20kc.20top.50pct.noskip.256sh.out \
                            > _dist/profile.20kc.20top.50pct.noskip.256sh.pdf

*/
