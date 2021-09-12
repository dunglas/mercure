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

func BenchmarkSubscriber(b *testing.B) {
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
	// Skip prioritity queue select and dump messages straight to out channel
	// Final solution will buffer live events until history is done replaying
	skipselectOpts := []string{"false", "true"}
	if opt := os.Getenv("SUB_TEST_SKIPSELECT"); len(opt) > 0 {
		skipselectOpts = []string{opt}
	} else {
		str = append(str, "skipselect %s")
	}
	// Which cache should the topic selector use (ristretto or lru)
	cacheOpts := []string{"ristretto", "lru"}
	if opt := os.Getenv("SUB_TEST_CACHE"); len(opt) > 0 {
		cacheOpts = []string{opt}
	} else {
		str = append(str, "cache %s")
	}
	// How many shards should the lru cache have (ristretto is hard coded to 256)
	shardOpts := []int{1, 256 /* 4096 */}
	if opt := os.Getenv("SUB_TEST_SHARDS"); len(opt) > 0 {
		shardOpts = []int{strInt(opt)}
	} else {
		str = append(str, "shards %d")
	}
	var arg []interface{}
	for _, topics := range topicOpts {
		var arg = arg
		if len(topicOpts) > 1 {
			arg = append(arg, topics)
		}
		for _, concurrency := range concurrencyOpts {
			var arg = arg
			if len(concurrencyOpts) > 1 {
				arg = append(arg, concurrency)
			}
			for _, matchPct := range matchPctOpts {
				var arg = arg
				if len(matchPctOpts) > 1 {
					arg = append(arg, matchPct)
				}
				for _, skipselect := range skipselectOpts {
					var arg = arg
					if len(skipselectOpts) > 1 {
						arg = append(arg, skipselect)
					}
					for _, cache := range cacheOpts {
						var arg = arg
						if len(cacheOpts) > 1 {
							arg = append(arg, cache)
						}
						for _, shards := range shardOpts {
							if cache == "ristretto" && shards != 256 {
								continue
							}
							var arg = arg
							if len(shardOpts) > 1 {
								arg = append(arg, shards)
							}
							subBenchSubscriber(b,
								topics,
								concurrency,
								matchPct,
								shards,
								cache,
								skipselect,
								fmt.Sprintf(strings.Join(str, " "), arg...),
							)
						}
					}
				}
			}
		}
	}
}

func strInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

func subBenchSubscriber(b *testing.B, topics, concurrency, matchPct, shards int, cache, skipselect string, testName string) {
	var tss *TopicSelectorStore
	var err error
	if cache == "lru" {
		tss, err = NewTopicSelectorStoreLru(1e5, int64(shards))
		if err != nil {
			panic(err)
		}
	} else {
		tss, err = NewTopicSelectorStore(1e6, 1000)
		if err != nil {
			panic(err)
		}
	}
	tss.skipSelect = skipselect == "true"
	var s = NewSubscriber("0e249241-6432-4ce1-b9b9-5d170163c253", zap.NewNop(), tss)
	s.Topics = make([]string, topics)
	tsMatch := make([]string, topics)
	tsNoMatch := make([]string, topics)
	for i := 0; i < topics; i++ {
		s.Topics[i] = fmt.Sprintf("/%d/{%d}", rand.Int(), rand.Int())
		tsNoMatch[i] = fmt.Sprintf("/%d/%d", rand.Int(), rand.Int())
		if topics/2 == i {
			// Insert matching topic half way through matching topic list to simulate match
			tsMatch[i] = strings.ReplaceAll(strings.ReplaceAll(s.Topics[i], "{", ""), "}", "")
		} else {
			tsMatch[i] = tsNoMatch[i]
		}
		// Warm cache
		for i := range tsMatch {
			for j := range s.Topics {
				tss.match(tsMatch[i], s.Topics[j])
			}
		}
	}
	go s.start()
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
				if i%100 <= matchPct {
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
