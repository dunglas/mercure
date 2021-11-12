package mercure

import (
	"strings"

	"github.com/kevburnsjr/skipfilter"
)

type SubscriberList struct {
	skipfilter *skipfilter.SkipFilter
}

func NewSubscriberList(size int) *SubscriberList {
	return &SubscriberList{
		skipfilter: skipfilter.New(func(s interface{}, topic interface{}) bool {
			p := strings.SplitN(topic.(string), "_", 2)
			if len(p) < 2 {
				return false
			}

			return s.(*Subscriber).Match(p[1], p[0] == "p")
		}, size),
	}
}

func (sc *SubscriberList) MatchAny(u *Update) (res []*Subscriber) {
	scopedTopics := make([]interface{}, len(u.Topics))
	for i, t := range u.Topics {
		if u.Private {
			scopedTopics[i] = "p_" + t
		} else {
			scopedTopics[i] = "_" + t
		}
	}

	for _, m := range sc.skipfilter.MatchAny(scopedTopics...) {
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
