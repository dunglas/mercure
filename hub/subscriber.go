package hub

import (
	"github.com/yosida95/uritemplate"
)

// Subscriber represents a client subscribed to a list of topics
type Subscriber struct {
	AllTargets     bool
	Targets        map[string]struct{}
	Topics         []string
	RawTopics      []string
	TemplateTopics []*uritemplate.Template
	LastEventID    string
	matchCache     map[string]bool
}

// NewSubscriber creates a subscriber
func NewSubscriber(allTargets bool, targets map[string]struct{}, topics []string, rawTopics []string, templateTopics []*uritemplate.Template, lastEventID string) *Subscriber {
	return &Subscriber{allTargets, targets, topics, rawTopics, templateTopics, lastEventID, make(map[string]bool)}
}

// IsAuthorized checks if the subscriber can access to at least one of the update's intended targets
// Don't forget to also call IsSubscribed
func (s *Subscriber) IsAuthorized(u *Update) bool {
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

// IsSubscribed checks if the subscriber has subscribed to this update
// Don't forget to also call IsAuthorized
func (s *Subscriber) IsSubscribed(u *Update) bool {
	for _, ut := range u.Topics {
		if match, ok := s.matchCache[ut]; ok {
			if match {
				return true
			}
			continue
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
