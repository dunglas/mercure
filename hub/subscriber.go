package hub

import (
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/yosida95/uritemplate"
)

// ErrSubscriberDisconnected is returned when the subscriber is disconnected.
var ErrSubscriberDisconnected = errors.New("subscriber: disconnected")

// ErrSubscriberNotAuthorized is returned when the subscriber is not authorized to receive this update.
var ErrSubscriberNotAuthorized = errors.New("subscriber: not authorized")

// ErrSubscriberNotSubscribed is returned when the subscriber is not subscribed to any topic of this update.
var ErrSubscriberNotSubscribed = errors.New("subscriber: not subscribed")

type updateSrc struct {
	sync.Mutex
	In     chan *Update
	buffer []*Update
}

// Subscriber represents a client subscribed to a list of topics.
type Subscriber struct {
	AllTargets     bool
	Targets        map[string]struct{}
	Topics         []string
	RawTopics      []string
	TemplateTopics []*uritemplate.Template
	LastEventID    string
	RemoteAddr     string

	HistorySrc updateSrc
	LiveSrc    updateSrc
	Out        chan *Update

	ClientDisconnect chan struct{}
	ServerDisconnect chan struct{}

	debug      bool
	matchCache map[string]bool
}

// NewSubscriber creates a subscriber.
func NewSubscriber(allTargets bool, targets map[string]struct{}, topics []string, rawTopics []string, templateTopics []*uritemplate.Template, lastEventID string, remoteAddr string, debug bool) *Subscriber {
	s := &Subscriber{
		allTargets,
		targets,
		topics,
		rawTopics,
		templateTopics,
		lastEventID,
		remoteAddr,

		updateSrc{},
		updateSrc{In: make(chan *Update)},
		make(chan *Update),

		make(chan struct{}),
		make(chan struct{}),

		debug,
		make(map[string]bool),
	}

	if lastEventID != "" {
		s.HistorySrc.In = make(chan *Update)
	}
	go s.start()

	return s
}

func (s *Subscriber) start() {
	for {
		select {
		case <-s.ClientDisconnect:
			return
		case <-s.ServerDisconnect:
			return
		case u, ok := <-s.HistorySrc.In:
			if ok {
				s.HistorySrc.buffer = append(s.HistorySrc.buffer, u)
				break
			}
			s.HistorySrc.In = nil
		case u := <-s.LiveSrc.In:
			s.LiveSrc.buffer = append(s.LiveSrc.buffer, u)
		case s.outChan() <- s.nextUpdate():
			if len(s.HistorySrc.buffer) > 0 {
				s.HistorySrc.buffer = s.HistorySrc.buffer[1:]
				break
			}

			s.LiveSrc.buffer = s.LiveSrc.buffer[1:]
		}
	}
}

func (s *Subscriber) outChan() chan *Update {
	if len(s.LiveSrc.buffer) > 0 || len(s.HistorySrc.buffer) > 0 {
		return s.Out
	}
	return nil
}

func (s *Subscriber) nextUpdate() *Update {
	// Always flush the history buffer first to preserve order
	if s.HistorySrc.In != nil || len(s.HistorySrc.buffer) > 0 {
		if len(s.HistorySrc.buffer) > 0 {
			return s.HistorySrc.buffer[0]
		}
		return nil
	}

	if len(s.LiveSrc.buffer) > 0 {
		return s.LiveSrc.buffer[0]
	}

	return nil
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

// Dispatch an update to the subscriber
func (s *Subscriber) Dispatch(u *Update, fromHistory bool) error {
	select {
	case <-s.ServerDisconnect:
		return ErrSubscriberDisconnected
	case <-s.ClientDisconnect:
		return ErrSubscriberDisconnected
	default:
	}

	fields := createBaseLogFields(s.debug, s.RemoteAddr, u, s)

	if !s.IsAuthorized(u) {
		log.WithFields(fields).Debug("Subscriber not authorized to receive this update (no targets matching)")
		return ErrSubscriberNotAuthorized
	}

	if !s.IsSubscribed(u) {
		log.WithFields(fields).Debug("Subscriber has not subscribed to this update (no topics matching)")
		return ErrSubscriberNotSubscribed
	}

	if fromHistory {
		s.HistorySrc.In <- u
		return nil
	}

	s.LiveSrc.In <- u
	return nil
}
