package hub

import (
	"regexp"
	"sync"
)

type subscribers struct {
	sync.RWMutex
	m map[chan *serializedUpdate]struct{}
}

// Subscriber represents a client subscribed to a list of topics
type Subscriber struct {
	AllTargets  bool
	Targets     map[string]struct{}
	Topics      []*regexp.Regexp
	LastEventID string
}

// CanReceive checks if the update can be dispatched according to the given criteria
func (s *Subscriber) CanReceive(u *Update) bool {
	return s.isAuthorized(u) && s.isSubscribed(u)
}

// isAuthorized checks if the subscriber can access to at least one of the update's intended targets
func (s *Subscriber) isAuthorized(u *Update) bool {
	if s.AllTargets || len(u.Targets) == 0 {
		return true
	}

	for t := range s.Targets {
		if _, ok := u.Targets[t]; ok {
			return true
		}
	}

	return false
}

// isSubscribedToUpdate checks if the subscriber has subscribed to this update
func (s *Subscriber) isSubscribed(u *Update) bool {
	// Add a global cache here
	for _, st := range s.Topics {
		for _, ut := range u.Topics {
			if st.MatchString(ut) {
				return true
			}
		}
	}

	return false
}
