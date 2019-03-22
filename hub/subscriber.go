package hub

import (
	"sync"

	"github.com/yosida95/uritemplate"
)

type subscribers struct {
	sync.RWMutex
	m map[chan *serializedUpdate]struct{}
}

// Subscriber represents a client subscribed to a list of topics
type Subscriber struct {
	AllTargets     bool
	Targets        map[string]struct{}
	RawTopics      []string
	TemplateTopics []*uritemplate.Template
	LastEventID    string
	matchCache     map[string]bool
}

// NewSubscriber creates a subscriber
func NewSubscriber(allTargets bool, targets map[string]struct{}, rawTopics []string, templateTopics []*uritemplate.Template, lastEventID string) *Subscriber {
	return &Subscriber{allTargets, targets, rawTopics, templateTopics, lastEventID, make(map[string]bool)}
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
	for _, ut := range u.Topics {
		if match, ok := s.matchCache[ut]; ok {
			return match
		}

		for _, rt := range s.RawTopics {
			if ut == rt {
				s.matchCache[ut] = true
				return true
			}
		}

		for _, tt := range s.TemplateTopics {
			if tt.Match(ut) != nil {
				s.matchCache[ut] = true
				return true
			}
		}

		s.matchCache[ut] = false
	}

	return false
}
