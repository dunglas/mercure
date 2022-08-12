package mercure

import (
	"encoding/base64"
	"sort"
	"strings"

	"github.com/kevburnsjr/skipfilter"
)

type SubscriberList struct {
	skipfilter *skipfilter.SkipFilter
}

func NewSubscriberList(size int) *SubscriberList {
	return &SubscriberList{
		skipfilter: skipfilter.New(func(s interface{}, filter interface{}) bool {
			var private bool

			encodedTopics := strings.Split(filter.(string), ":")
			topics := make([]string, len(encodedTopics))
			for i, encodedTopic := range encodedTopics {
				decoded, err := base64.StdEncoding.DecodeString(encodedTopic)
				if err != nil {
					return false
				}

				p := strings.SplitN(string(decoded), "_", 2)
				if len(p) < 2 {
					return false
				}

				if p[0] == "p" {
					private = true
				}
				topics[i] = p[1]
			}

			return s.(*Subscriber).MatchTopics(topics, private)
		}, size),
	}
}

func (sc *SubscriberList) MatchAny(u *Update) (res []*Subscriber) {
	encodedTopics := make([]string, len(u.Topics))
	for i, t := range u.Topics {
		if u.Private {
			encodedTopics[i] = base64.StdEncoding.EncodeToString([]byte("p_" + t))
		} else {
			encodedTopics[i] = base64.StdEncoding.EncodeToString([]byte("_" + t))
		}
	}

	sort.Strings(encodedTopics)

	for _, m := range sc.skipfilter.MatchAny(strings.Join(encodedTopics, ":")) {
		res = append(res, m.(*Subscriber))
	}

	return
}

func (sc *SubscriberList) Walk(start uint64, callback func(s *Subscriber) bool) uint64 {
	return sc.skipfilter.Walk(start, func(val interface{}) bool {
		return callback(val.(*Subscriber))
	})
}

func (sc *SubscriberList) Add(s *Subscriber) {
	sc.skipfilter.Add(s)
}

func (sc *SubscriberList) Remove(s *Subscriber) {
	sc.skipfilter.Remove(s)
}

func (sc *SubscriberList) Len() int {
	return sc.skipfilter.Len()
}
