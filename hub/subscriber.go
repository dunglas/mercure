package hub

import (
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/yosida95/uritemplate"
)

type updateSource struct {
	In     chan *Update
	buffer []*Update
}

// Subscriber represents a client subscribed to a list of topics.
type Subscriber struct {
	ID      string
	History updateSource
	Live    updateSource
	Out     chan *Update

	disconnected   chan struct{}
	claims         *claims
	allTargets     bool
	targets        map[string]struct{}
	topics         []string
	escapedTopics  []string
	rawTopics      []string
	templateTopics []*uritemplate.Template
	lastEventID    string
	remoteAddr     string
	remoteHost     string
	debug          bool

	logFields  log.Fields
	matchCache map[string]bool
}

func newSubscriber() *Subscriber {
	id := uuid.Must(uuid.NewV4()).String()
	return &Subscriber{
		ID:           id,
		History:      updateSource{},
		Live:         updateSource{In: make(chan *Update)},
		Out:          make(chan *Update),
		disconnected: make(chan struct{}),
		logFields:    log.Fields{"subscriber_id": id},
		matchCache:   make(map[string]bool),
	}
}

func (s *Subscriber) start() {
	for {
		select {
		case <-s.disconnected:
			return
		case u, ok := <-s.History.In:
			if !ok {
				s.History.In = nil
				break
			}
			if s.CanDispatch(u) {
				s.History.buffer = append(s.History.buffer, u)
			}
		case u := <-s.Live.In:
			if s.CanDispatch(u) {
				s.Live.buffer = append(s.Live.buffer, u)
			}
		case s.outChan() <- s.nextUpdate():
			if len(s.History.buffer) > 0 {
				s.History.buffer = s.History.buffer[1:]
				break
			}

			s.Live.buffer = s.Live.buffer[1:]
		}
	}
}

func (s *Subscriber) outChan() chan *Update {
	if len(s.Live.buffer) > 0 || len(s.History.buffer) > 0 {
		return s.Out
	}
	return nil
}

func (s *Subscriber) nextUpdate() *Update {
	// Always flush the history buffer first to preserve order
	if s.History.In != nil || len(s.History.buffer) > 0 {
		if len(s.History.buffer) > 0 {
			return s.History.buffer[0]
		}
		return nil
	}

	if len(s.Live.buffer) > 0 {
		return s.Live.buffer[0]
	}

	return nil
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
	if s.allTargets || len(u.Targets) == 0 {
		return true
	}

	for t := range s.targets {
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

		for _, rt := range s.rawTopics {
			if ut == rt {
				s.matchCache[ut] = true
				return true
			}
		}

		for _, tt := range s.templateTopics {
			if tt.Match(ut) != nil {
				s.matchCache[ut] = true
				return true
			}
		}

		s.matchCache[ut] = false
	}

	return false
}

// Dispatch an update to the subscriber.
func (s *Subscriber) Dispatch(u *Update, fromHistory bool) bool {
	var in chan<- *Update
	if fromHistory {
		in = s.History.In
	} else {
		in = s.Live.In
	}

	select {
	case <-s.disconnected:
		return false
	case in <- u:
	}

	return true
}

func (s *Subscriber) Disconnect() {
	select {
	case <-s.disconnected:
		return
	default:
	}

	close(s.disconnected)
}
