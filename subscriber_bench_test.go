package mercure

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
)

func subscribeBenchmarkHelper(b *testing.B, subBench func(b *testing.B, topics, concurrency, matchPct int, testName string)) {
	b.Helper()

	var str []string

	// How many topics and topicselectors do each subscriber and update contain (same value for both)
	topicOpts := []int{1, 5, 10}
	if opt := os.Getenv("SUB_TEST_TOPICS"); len(opt) > 0 {
		topicOpts = []int{strInt(opt)}
	} else {
		str = append(str, "topics %d")
	}

	// How many concurrent subscribers
	concurrencyOpts := []int{100, 1000, 5000, 20000}
	if opt := os.Getenv("SUB_TEST_CONCURRENCY"); len(opt) > 0 {
		concurrencyOpts = []int{strInt(opt)}
	} else {
		str = append(str, "concurrency %d")
	}

	// What percentage of messages are delivered to a subscriber (ie 10 = 10% CanDispatch true)
	matchPctOpts := []int{1, 10, 100}
	if opt := os.Getenv("SUB_TEST_MATCHPCT"); len(opt) > 0 {
		matchPctOpts = []int{strInt(opt)}
	} else {
		str = append(str, "matchpct %d")
	}

	var arg []interface{}
	for _, topics := range topicOpts {
		arg := arg
		if len(topicOpts) > 1 {
			arg = append(arg, topics)
		}
		for _, concurrency := range concurrencyOpts {
			arg := arg
			if len(concurrencyOpts) > 1 {
				arg = append(arg, concurrency)
			}
			for _, matchPct := range matchPctOpts {
				arg := arg
				if len(matchPctOpts) > 1 {
					arg = append(arg, matchPct)
				}
				subBench(b,
					topics,
					concurrency,
					matchPct,
					fmt.Sprintf(strings.Join(str, " "), arg...),
				)
			}
		}
	}
}

func BenchmarkSubscriber(b *testing.B) {
	subscribeBenchmarkHelper(b, subBenchSubscriber)
}

func strInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}

	return n
}

func subBenchSubscriber(b *testing.B, topics, concurrency, matchPct int, testName string) {
	b.Helper()

	s := NewSubscriber("0e249241-6432-4ce1-b9b9-5d170163c253", zap.NewNop())
	ts := make([]string, topics)
	tsMatch := make([]string, topics)
	tsNoMatch := make([]string, topics)
	for i := 0; i < topics; i++ {
		ts[i] = fmt.Sprintf("/%d/{%d}", rand.Int(), rand.Int())      //nolint:gosec
		tsNoMatch[i] = fmt.Sprintf("/%d/%d", rand.Int(), rand.Int()) //nolint:gosec
		if topics/2 == i {
			// Insert matching topic half way through matching topic list to simulate match
			tsMatch[i] = strings.ReplaceAll(strings.ReplaceAll(ts[i], "{", ""), "}", "")
		} else {
			tsMatch[i] = tsNoMatch[i]
		}
	}
	s.SetTopics(ts, nil)
	defer s.Disconnect()
	ctx, done := context.WithCancel(context.Background())
	defer done()
	for i := 0; i < 1; i++ {
		go func() {
			for {
				select {
				case <-s.out:
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
				if i%100 < matchPct {
					s.Dispatch(&Update{Topics: tsMatch}, i%2 == 0 /* half history, half live */)
				} else {
					s.Dispatch(&Update{Topics: tsNoMatch}, i%2 == 0 /* half history, half live */)
				}
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
SUB_TEST_SKIPSELECT=false \
SUB_TEST_CACHE=ristretto \
SUB_TEST_SHARDS=256 \
go test -bench=. -run=BenchmarkSubscriber -benchmem -count 5 | tee _dist/output.5kc.noskip.ristretto.256.txt

SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_SKIPSELECT=true \
SUB_TEST_CACHE=lru \
SUB_TEST_SHARDS=256 \
go test -bench=. -run=BenchmarkSubscriber -benchmem -count 5 | tee _dist/output.5kc.skip.lru.256.txt

benchstat _dist/output.5kc.noskip.ristretto.256.txt \
          _dist/output.5kc.skip.lru.256.txt \
        > _dist/diff-cache.5kc.256.txt


# --- Generating a cpu call graph ---

SUB_TEST_CONCURRENCY=20000 \
SUB_TEST_TOPICS=20 \
SUB_TEST_MATCHPCT=50 \
SUB_TEST_SKIPSELECT=false \
SUB_TEST_CACHE=ristretto \
SUB_TEST_SHARDS=256 \
go test -bench=. -run=BenchmarkSubscriber -cpuprofile _dist/profile.20kc.20top.50pct.noskip.ristretto.256sh.out -benchmem

go build -o _dist/bin

go tool pprof --pdf _dist/bin _dist/profile.20kc.20top.50pct.noskip.ristretto.256sh.out \
                            > _dist/profile.20kc.20top.50pct.noskip.ristretto.256sh.pdf

*/
