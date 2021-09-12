package mercure

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"math/rand"
	"os"
	"strings"

	"go.uber.org/zap"
)

func BenchmarkSubscriber(b *testing.B) {
	var str []string
	topicOpts := []int{1, 5, 10, 20}
	if opt := os.Getenv("SUB_TEST_TOPICS"); len(opt) > 0 {
		topicOpts = []int{strInt(opt)}
	} else {
		str = append(str, "topics %d")
	}
	concurrencyOpts := []int{1, 10, 100, 1000, 10000}
	if opt := os.Getenv("SUB_TEST_CONCURRENCY"); len(opt) > 0 {
		concurrencyOpts = []int{strInt(opt)}
	} else {
		str = append(str, "concurrency %d")
	}
	matchPctOpts := []int{1, 10, 50, 100}
	if opt := os.Getenv("SUB_TEST_MATCHPCT"); len(opt) > 0 {
		matchPctOpts = []int{strInt(opt)}
	} else {
		str = append(str, "matchPct %d")
	}
	cacheOpts := []string{"ristretto", "lru"}
	if opt := os.Getenv("SUB_TEST_CACHE"); len(opt) > 0 {
		cacheOpts = []string{opt}
	} else {
		str = append(str, "cache %s")
	}
	shardOpts := []int{1, 16, 256, 4096}
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
				for _, cache := range cacheOpts {
					var arg = arg
					if len(cache) > 1 {
						arg = append(arg, cache)
					}
					for _, shards := range shardOpts {
						var arg = arg
						if len(shardOpts) > 1 {
							arg = append(arg, shards)
						}
						if cache == "ristretto" && shards != 256 {
							continue
						}
						subBenchSubscriber(b, topics, concurrency, matchPct, shards, cache, fmt.Sprintf(strings.Join(str, " "), arg...))
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

func subBenchSubscriber(b *testing.B, topics, concurrency, matchPct, shards int, cache, testName string) {
	b.Run(testName, func(b *testing.B) {
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
		var s = NewSubscriber("0e249241-6432-4ce1-b9b9-5d170163c253", zap.NewNop(), tss)
		s.Topics = make([]string, topics)
		tsMatch := make([]string, topics)
		tsNoMatch := make([]string, topics)
		for i := 0; i < topics; i++ {
			s.Topics[i] = fmt.Sprintf("/%d/{%d}", rand.Int(), rand.Int())
			tsNoMatch[i] = fmt.Sprintf("/%d/%d", rand.Int(), rand.Int())
			if topics / 2 == i {
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
		var wg sync.WaitGroup
		wg.Add(concurrency)
		b.ResetTimer()
		for i := 0; i < concurrency; i++ {
			go func() {
				for i := 0; i < b.N/concurrency; i++ {
					if i % 100 <= matchPct {
						s.Dispatch(&Update{Topics: tsMatch}, i%2 == 0 /* half history, half live */)
					} else {
						s.Dispatch(&Update{Topics: tsNoMatch}, i%2 == 0 /* half history, half live */)
					}
				}
				wg.Done()
			}()
		}
		// Wait for dispatch generator to finish
		wg.Wait()
	})
}

/* --- test.sh --- These are the commands required to produce desired scheduler contention and record output.

#!/usr/bin/sh

mkdir -p _dist

echo "50 post"
SUB_TEST_MATCHPCT=50 \
SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_DISPATCH=post \
go test -bench=. -run=BenchmarkSubscriber -cpuprofile _dist/profile.50pct.5000c.post.out -count 5 | tee _dist/profile.50pct.5000c.post.txt

echo "50 pre"
SUB_TEST_MATCHPCT=50 \
SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_DISPATCH=pre \
go test -bench=. -run=BenchmarkSubscriber -cpuprofile _dist/profile.50pct.5000c.pre.out -count 5 | tee _dist/profile.50pct.5000c.pre.txt

benchstat _dist/profile.50pct.5000c.post.txt _dist/profile.50pct.5000c.pre.txt > _dist/profile.50pct.5000c.diff.txt

echo "10 post"
SUB_TEST_MATCHPCT=10 \
SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_DISPATCH=post \
go test -bench=. -run=BenchmarkSubscriber -cpuprofile _dist/profile.10pct.5000c.post.out -count 5 | tee _dist/profile.10pct.5000c.post.txt

echo "10 pre"
SUB_TEST_MATCHPCT=10 \
SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_DISPATCH=pre \
go test -bench=. -run=BenchmarkSubscriber -cpuprofile _dist/profile.10pct.5000c.pre.out -count 5 | tee _dist/profile.10pct.5000c.pre.txt

benchstat _dist/profile.10pct.5000c.post.txt _dist/profile.10pct.5000c.pre.txt > _dist/profile.10pct.5000c.diff.txt

echo "1 post"
SUB_TEST_MATCHPCT=1 \
SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_DISPATCH=post \
go test -bench=. -run=BenchmarkSubscriber -cpuprofile _dist/profile.1pct.5000c.post.out -count 5 | tee _dist/profile.1pct.5000c.post.txt

echo "1 pre"
SUB_TEST_MATCHPCT=1 \
SUB_TEST_CONCURRENCY=5000 \
SUB_TEST_DISPATCH=pre \
go test -bench=. -run=BenchmarkSubscriber -cpuprofile _dist/profile.1pct.5000c.pre.out -count 5 | tee _dist/profile.1pct.5000c.pre.txt

benchstat _dist/profile.1pct.5000c.post.txt _dist/profile.1pct.5000c.pre.txt > _dist/profile.1pct.5000c.diff.txt

*/