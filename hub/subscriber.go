package hub

import (
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/yosida95/uritemplate"
)

type updateSource struct {
	in     chan *Update
	buffer []*Update
}

// Subscriber represents a client subscribed to a list of topics.
type Subscriber struct {
	ID             string
	Claims         *claims
	Targets        map[string]struct{}
	Topics         []string
	EscapedTopics  []string
	RawTopics      []string
	TemplateTopics []*uritemplate.Template
	LastEventID    string
	RemoteAddr     string
	RemoteHost     string
	LogFields      log.Fields
	AllTargets     bool
	Debug          bool

	out          chan *Update
	disconnected chan struct{}
	matchCache   map[string]bool
	history      updateSource
	live         updateSource
}

func newSubscriber(lastEventID string) *Subscriber {
	id := uuid.Must(uuid.NewV4()).String()
	s := &Subscriber{
		ID:          id,
		LastEventID: lastEventID,
		LogFields: log.Fields{
			"subscriber_id": id,
			"last_event_id": lastEventID,
		},
		history:      updateSource{},
		live:         updateSource{in: make(chan *Update)},
		out:          make(chan *Update),
		disconnected: make(chan struct{}),
		matchCache:   make(map[string]bool),
	}

	if lastEventID != "" {
		s.history.in = make(chan *Update)
	}

	return s
}

// start stores incoming updates in an history and a live buffer and dispatch them.
// Updates coming from the history are always dispatched first.
func (s *Subscriber) start() {
	for {
		select {
		case <-s.disconnected:
			return
		case u, ok := <-s.history.in:
			if !ok {
				s.history.in = nil
				break
			}
			if s.CanDispatch(u) {
				s.history.buffer = append(s.history.buffer, u)
			}
		case u := <-s.live.in:
			if s.CanDispatch(u) {
				s.live.buffer = append(s.live.buffer, u)
			}
		case s.outChan() <- s.nextUpdate():
			if len(s.history.buffer) > 0 {
				s.history.buffer = s.history.buffer[1:]
				break
			}

			s.live.buffer = s.live.buffer[1:]
		}
	}
}

// outChan returns the out channel if buffers aren't empty, or nil to block.
func (s *Subscriber) outChan() chan<- *Update {
	if len(s.live.buffer) > 0 || len(s.history.buffer) > 0 {
		return s.out
	}
	return nil
}

// nextUpdate returns the next update to dispatch.
// The history is always entirely flushed before starting to dispatch live updates.
func (s *Subscriber) nextUpdate() *Update {
	// Always flush the history buffer first to preserve order
	if s.history.in != nil || len(s.history.buffer) > 0 {
		if len(s.history.buffer) > 0 {
			return s.history.buffer[0]
		}
		return nil
	}

	if len(s.live.buffer) > 0 {
		return s.live.buffer[0]
	}

	return nil
}

// Dispatch an update to the subscriber.
func (s *Subscriber) Dispatch(u *Update, fromHistory bool) bool {
	var in chan<- *Update
	if fromHistory {
		in = s.history.in
	} else {
		in = s.live.in
	}

	select {
	case <-s.disconnected:
		return false
	case in <- u:
	}

	return true
}

// Receive returns a chan when incoming updates are dispatched.
func (s *Subscriber) Receive() <-chan *Update {
	return s.out
}

// HistoryDispatched must be called when all messages coming from the history have been dispatched.
func (s *Subscriber) HistoryDispatched() {
	close(s.history.in)
}

// Disconnect disconnects the subscriber.
func (s *Subscriber) Disconnect() {
	select {
	case <-s.disconnected:
		return
	default:
	}

	close(s.disconnected)
}

// Disconnected allows to check if the subscriber is disconnected.
func (s *Subscriber) Disconnected() <-chan struct{} {
	return s.disconnected
}

// CanDispatch checks if an update can be dispatched to this subsriber.
func (s *Subscriber) CanDispatch(u *Update) bool {
	if !s.IsAuthorized(u) {
		log.WithFields(createFields(u, s)).Debug("Subscriber not authorized to receive this update (no targets matching)")
		return false
	}

	if !s.IsSubscribed(u) {
		log.WithFields(createFields(u, s)).Debug("Subscriber has not subscribed to this update (no topics matching)")
		return false
	}

	return true
}

// IsAuthorized checks if the subscriber can access to at least one of the update's intended targets.
// Don't forget to also call IsSubscribed.
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

// IsSubscribed checks if the subscriber has subscribed to this update.
// Don't forget to also call IsAuthorized.
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
