package mercure

import (
	"github.com/kevburnsjr/skipfilter"
)

type filter struct {
	topics  []string
	private bool
}

type SubscriberList struct {
	skipfilter *skipfilter.SkipFilter
}

func NewSubscriberList(size int) *SubscriberList {
	return &SubscriberList{
		skipfilter: skipfilter.New(func(s interface{}, fil interface{}) bool {
			f := fil.(*filter)

			return s.(*Subscriber).MatchTopics(f.topics, f.private)
		}, size),
	}
}

func (sc *SubscriberList) MatchAny(u *Update) (res []*Subscriber) {
	f := &filter{u.Topics, u.Private}

	for _, m := range sc.skipfilter.MatchAny(f) {
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
