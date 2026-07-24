package mercure

import "testing"

// BenchmarkMatchMatcher exercises the per-matcher hot path (Subscriber.Match ->
// matchesAny -> matches) for each matcher type, isolating the cost the new
// two-matcher implementation adds over a plain exact comparison. Branch-only:
// main has no matches.
func BenchmarkMatchMatcher(b *testing.B) {
	const base = "https://example.com"

	topics := []string{"https://example.com/books/1"}

	cached, err := NewTopicMatcherStore(DefaultTopicMatcherStoreCacheSize)
	if err != nil {
		b.Fatal(err)
	}

	if err := cached.setBaseURL(base); err != nil {
		b.Fatal(err)
	}

	nocache := &TopicMatcherStore{}
	if err := nocache.setBaseURL(base); err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name  string
		store *TopicMatcherStore
		m     TopicMatcher
	}{
		{"exact-hit", cached, TopicMatcher{MatcherTypeExact, "https://example.com/books/1"}},
		{"exact-miss", cached, TopicMatcher{MatcherTypeExact, "https://example.com/books/2"}},
		{"wildcard", cached, TopicMatcher{MatcherTypeExact, "*"}},
		{"urlpattern-hit-cached", cached, TopicMatcher{MatcherTypeURLPattern, "https://example.com/books/:id"}},
		{"urlpattern-miss-cached", cached, TopicMatcher{MatcherTypeURLPattern, "https://example.com/authors/:id"}},
		{"urlpattern-hit-nocache", nocache, TopicMatcher{MatcherTypeURLPattern, "https://example.com/books/:id"}},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()

			var ok bool
			for range b.N {
				ok = c.store.matches(topics, c.m)
			}

			runtimeSink = ok
		})
	}
}

// runtimeSink defeats dead-code elimination of the match result.
var runtimeSink bool //nolint:gochecknoglobals
