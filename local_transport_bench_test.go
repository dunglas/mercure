package mercure

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"sync"
	"testing"

	"go.uber.org/zap"
)

func BenchmarkLocalTransport(b *testing.B) {
	subscribeBenchmarkHelper(b, subBenchLocalTransport)
}

func subBenchLocalTransport(b *testing.B, topics, concurrency, matchPct int, testName string) {
	b.Helper()

	tr, err := NewLocalTransport(&url.URL{Scheme: "local"}, zap.NewNop())
	if err != nil {
		panic(err)
	}
	defer tr.Close()
	top := make([]string, topics)
	tsMatch := make([]string, topics)
	tsNoMatch := make([]string, topics)
	for i := 0; i < topics; i++ {
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
	out := make(chan *Update, 50000)
	for i := 0; i < concurrency; i++ {
		s := NewSubscriber("", zap.NewNop())
		if i%100 < matchPct {
			s.SetTopics(tsMatch, nil)
		} else {
			s.SetTopics(tsNoMatch, nil)
		}
		s.out = out
		tr.AddSubscriber(s)
	}
	ctx, done := context.WithCancel(context.Background())
	defer done()
	for i := 0; i < 1; i++ {
		go func() {
			for {
				select {
				case <-out:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	b.SetParallelism(concurrency)
	b.Run(testName, func(b *testing.B) {
		var wg sync.WaitGroup
		wg.Add(concurrency)
		b.RunParallel(func(pb *testing.PB) {
			for i := 0; pb.Next(); i++ {
				tr.Dispatch(&Update{Topics: top})
			}
		})
		wg.Done()
	})
}

/* --- test.sh ---
These are example commands that can be used to run subsets of this test for analysis.
Omission of any environment variable causes the test to enumate a few meaningful options.

#!/usr/bin/sh

set -e

mkdir -p _dist

# --- Generating a diff ---

SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_CACHE=ristretto \
SUB_TEST_SHARDS=256 \
go test -bench=. -run=BenchmarkLocalTransport -benchmem -count 5 | tee _dist/output.5kc.noskip.ristretto.256.txt

SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_CACHE=lru \
SUB_TEST_SHARDS=256 \
go test -bench=. -run=BenchmarkLocalTransport -benchmem -count 5 | tee _dist/output.5kc.skip.lru.256.txt

benchstat _dist/output.5kc.noskip.ristretto.256.txt \
          _dist/output.5kc.skip.lru.256.txt \
        > _dist/diff-cache.5kc.256.txt


# --- Generating a cpu call graph ---

SUB_TEST_CONCURRENCY=20000 \
SUB_TEST_TOPICS=20 \
SUB_TEST_MATCHPCT=50 \
SUB_TEST_CACHE=ristretto \
SUB_TEST_SHARDS=256 \
go test -bench=. -run=BenchmarkLocalTransport -cpuprofile _dist/profile.20kc.20top.50pct.noskip.ristretto.256sh.out -benchmem

go build -o _dist/bin

go tool pprof --pdf _dist/bin _dist/profile.20kc.20top.50pct.noskip.ristretto.256sh.out \
                            > _dist/profile.20kc.20top.50pct.noskip.ristretto.256sh.pdf

*/
