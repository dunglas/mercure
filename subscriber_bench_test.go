package mercure

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
)

func subscribeBenchmarkHelper(b *testing.B, subBench func(b *testing.B, topics, concurrency, matchPct int, testName string)) {
	b.Helper()

	// How many topics and topics electors do each subscriber and update contain (same value for both)
	var topicOpts []int
	if opt := os.Getenv("SUB_TEST_TOPICS"); opt != "" {
		topicOpts = parseIntsEnvVar(opt)
	} else {
		topicOpts = []int{1, 5, 10}
	}

	// How many concurrent subscribers
	var concurrencyOpts []int
	if opt := os.Getenv("SUB_TEST_CONCURRENCY"); opt != "" {
		concurrencyOpts = parseIntsEnvVar(opt)
	} else {
		concurrencyOpts = []int{100, 1000, 5000, 20000}
	}

	// What percentage of messages are delivered to a subscriber (ie 10 = 10% CanDispatch true)
	var matchPctOpts []int
	if opt := os.Getenv("SUB_TEST_MATCHPCT"); opt != "" {
		matchPctOpts = parseIntsEnvVar(opt)
	} else {
		matchPctOpts = []int{1, 10, 100}
	}

	for _, topics := range topicOpts {
		for _, concurrency := range concurrencyOpts {
			for _, matchPct := range matchPctOpts {
				subBench(b,
					topics,
					concurrency,
					matchPct,
					fmt.Sprintf("%d-topics:%d-concurrency:%d-matchpct", topics, concurrency, matchPct),
				)
			}
		}
	}
}

func BenchmarkSubscriber(b *testing.B) {
	subscribeBenchmarkHelper(b, subBenchSubscriber)
}

func parseIntsEnvVar(s string) (res []int) {
	parts := strings.Split(s, ",")
	res = make([]int, len(parts))

	for i, part := range parts {
		v, err := strconv.Atoi(part)
		if err != nil {
			panic(err)
		}

		res[i] = v
	}

	return res
}

func subBenchSubscriber(b *testing.B, topics, concurrency, matchPct int, testName string) {
	b.Helper()

	s := NewLocalSubscriber("0e249241-6432-4ce1-b9b9-5d170163c253", slog.Default(), &TopicSelectorStore{})
	ts := make([]string, topics)
	tsMatch := make([]string, topics)
	ctx := b.Context()

	tsNoMatch := make([]string, topics)
	for i := range topics {
		ts[i] = fmt.Sprintf("/%d/{%d}", rand.Int(), rand.Int()) //nolint:gosec

		tsNoMatch[i] = fmt.Sprintf("/%d/%d", rand.Int(), rand.Int()) //nolint:gosec
		if topics/2 == i {
			// Insert matching topic half-way through matching topic list to simulate match
			tsMatch[i] = strings.ReplaceAll(strings.ReplaceAll(ts[i], "{", ""), "}", "")
		} else {
			tsMatch[i] = tsNoMatch[i]
		}
	}

	s.SetTopics(ts, nil)
	b.Cleanup(s.Disconnect)

	ctx, done := context.WithCancel(ctx)
	defer done()

	for range 1 {
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
		b.RunParallel(func(pb *testing.PB) {
			for i := 0; pb.Next(); i++ {
				if i%100 < matchPct {
					s.Dispatch(ctx, &Update{Topics: tsMatch}, i%2 == 0 /* half history, half live */)
				} else {
					s.Dispatch(ctx, &Update{Topics: tsNoMatch}, i%2 == 0 /* half history, half live */)
				}
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
	go test -bench=. -run=BenchmarkSubscriber -cpuprofile profile.out -benchmem
go tool pprof --pdf _dist/bin profile.out > profile.pdf
*/
