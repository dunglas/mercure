package mercure

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"sync"

	"github.com/dunglas/skipfilter"
)

type SubscriberList struct {
	skipfilter *skipfilter.SkipFilter[*LocalSubscriber, filterKey]

	// The digest used as cache key cannot be decoded back into the topics the filter needs.
	inFlightMu sync.RWMutex
	inFlight   map[filterKey]inFlightFilter
}

type filterKey [sha256.Size]byte

type inFlightFilter struct {
	topics  []string
	private bool
	refs    int
}

// DefaultSubscriberListCacheSize is the default size of the skipfilter cache.
//
// Keys are fixed-size digests, so a full cache of 100,000 filters with no
// subscribers uses about 25MB whatever the size of the update topics.
const DefaultSubscriberListCacheSize = 100_000

func NewSubscriberList(cacheSize int) *SubscriberList {
	sl := &SubscriberList{inFlight: make(map[filterKey]inFlightFilter)}

	sl.skipfilter = skipfilter.New(func(s *LocalSubscriber, k filterKey) bool {
		sl.inFlightMu.RLock()
		f := sl.inFlight[k]
		sl.inFlightMu.RUnlock()

		return s.MatchTopics(f.topics, f.private)
	}, cacheSize)

	return sl
}

// newFilterKey returns one digest per topic set and private flag; SHA-256 makes the collision that
// would match an update against another topic set's subscribers computationally infeasible.
func newFilterKey(topics []string, private bool) filterKey {
	// Sort a copy: topics can be the Update's own Topics, read concurrently.
	var sortedBuf [16]string

	sorted := append(sortedBuf[:0], topics...)
	slices.Sort(sorted)

	var inputBuf [512]byte

	input := inputBuf[:1]
	if private {
		input[0] = 1
	}

	for _, t := range sorted {
		// Length-prefixed so that topic boundaries are part of the hashed input.
		input = binary.AppendUvarint(input, uint64(len(t)))
		input = append(input, t...)
	}

	return sha256.Sum256(input)
}

func (sl *SubscriberList) MatchAny(u *Update) []*LocalSubscriber {
	k := newFilterKey(u.Topics, u.Private)

	sl.retain(k, u)
	defer sl.release(k)

	return sl.skipfilter.MatchAny(k)
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

func (sl *SubscriberList) retain(k filterKey, u *Update) {
	sl.inFlightMu.Lock()
	defer sl.inFlightMu.Unlock()

	f, ok := sl.inFlight[k]
	if !ok {
		f = inFlightFilter{topics: u.Topics, private: u.Private}
	}

	f.refs++
	sl.inFlight[k] = f
}

func (sl *SubscriberList) release(k filterKey) {
	sl.inFlightMu.Lock()
	defer sl.inFlightMu.Unlock()

	f := sl.inFlight[k]

	f.refs--
	if f.refs == 0 {
		delete(sl.inFlight, k)

		return
	}

	sl.inFlight[k] = f
}
