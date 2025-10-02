package mercure

import (
	"sort"
	"strings"

	"github.com/dunglas/skipfilter"
)

type SubscriberList struct {
	skipfilter *skipfilter.SkipFilter[*LocalSubscriber, string]
}

// We choose a delimiter and an escape character which are unlikely to be used.
const (
	escape = '\x00'
	delim  = '\x01'
)

//nolint:gochecknoglobals
var replacer = strings.NewReplacer(
	string(escape), string([]rune{escape, escape}),
	string(delim), string([]rune{escape, delim}),
)

func NewSubscriberList() *SubscriberList {
	return &SubscriberList{
		skipfilter: skipfilter.New[*LocalSubscriber, string](func(s *LocalSubscriber, filter string) bool {
			return s.MatchTopics(decode(filter))
		}, 0),
	}
}

func encode(topics []string, private bool) string {
	sort.Strings(topics)

	parts := make([]string, len(topics)+1)
	if private {
		parts[0] = "1"
	} else {
		parts[0] = "0"
	}

	for i, t := range topics {
		parts[i+1] = replacer.Replace(t)
	}

	return strings.Join(parts, string(delim))
}

func decode(f string) (topics []string, private bool) {
	var (
		privateExtracted, inEscape bool
		builder                    strings.Builder
	)

	for _, char := range f {
		if inEscape {
			builder.WriteRune(char)

			inEscape = false

			continue
		}

		switch char {
		case escape:
			inEscape = true

		case delim:
			if !privateExtracted {
				private = builder.String() == "1"
				builder.Reset()

				privateExtracted = true

				break
			}

			topics = append(topics, builder.String())
			builder.Reset()

		default:
			builder.WriteRune(char)
		}
	}

	topics = append(topics, builder.String())

	return topics, private
}

func (sl *SubscriberList) MatchAny(u *Update) (res []*LocalSubscriber) {
	return append(res, sl.skipfilter.MatchAny(encode(u.Topics, u.Private))...)
}

func (sl *SubscriberList) Walk(start uint64, callback func(s *LocalSubscriber) bool) uint64 {
	return sl.skipfilter.Walk(start, func(val *LocalSubscriber) bool {
		return callback(val)
	})
}

func (sl *SubscriberList) Add(s *LocalSubscriber) {
	sl.skipfilter.Add(s)
}

func (sl *SubscriberList) Remove(s *LocalSubscriber) {
	sl.skipfilter.Remove(s)
}

func (sl *SubscriberList) Len() int {
	return sl.skipfilter.Len()
}
