package mercure

import (
	"bytes"
	"encoding/gob"
	"sort"
	"strings"

	"github.com/kevburnsjr/skipfilter"
)

type filter struct {
	Topics  []string
	Private bool
}

type SubscriberList struct {
	skipfilter *skipfilter.SkipFilter
}

func NewSubscriberList(size int) *SubscriberList {
	return &SubscriberList{
		skipfilter: skipfilter.New(func(s interface{}, fi interface{}) bool {
			dec := gob.NewDecoder(strings.NewReader(fi.(string)))
			f := filter{}
			if err := dec.Decode(&f); err != nil {
				panic(err)
			}

			return s.(*Subscriber).MatchTopics(f.Topics, f.Private)
		}, size),
	}
}

func (sc *SubscriberList) MatchAny(u *Update) (res []*Subscriber) {
	f := filter{u.Topics, u.Private}
	sort.Strings(f.Topics)

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(f); err != nil {
		panic(err)
	}

	for _, m := range sc.skipfilter.MatchAny(buf.String()) {
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
