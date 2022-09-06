package mercure

import (
	"sort"
	"strings"

	"github.com/kevburnsjr/skipfilter"
)

type SubscriberList struct {
	skipfilter *skipfilter.SkipFilter
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

func NewSubscriberList(size int) *SubscriberList {
	return &SubscriberList{
		skipfilter: skipfilter.New(func(s interface{}, filter interface{}) bool {
			return s.(*Subscriber).MatchTopics(decode(filter.(string)))
		}, size),
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

func (sl *SubscriberList) MatchAny(u *Update) (res []*Subscriber) {
	for _, m := range sl.skipfilter.MatchAny(encode(u.Topics, u.Private)) {
		res = append(res, m.(*Subscriber))
	}

	return
}

func (sl *SubscriberList) Walk(start uint64, callback func(s *Subscriber) bool) uint64 {
	return sl.skipfilter.Walk(start, func(val interface{}) bool {
		return callback(val.(*Subscriber))
	})
}

func (sl *SubscriberList) Add(s *Subscriber) {
	sl.skipfilter.Add(s)
}

func (sl *SubscriberList) Remove(s *Subscriber) {
	sl.skipfilter.Remove(s)
}

func (sl *SubscriberList) Len() int {
	return sl.skipfilter.Len()
}
