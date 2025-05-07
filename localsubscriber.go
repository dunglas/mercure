package mercure

import (
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
)

// LocalSubscriber represents a client subscribed to a list of topics on the current hub.
type LocalSubscriber struct {
	Subscriber

	disconnected        atomic.Uint32
	out                 chan *Update
	outMutex            sync.Mutex
	responseLastEventID chan string
	ready               atomic.Uint32
	liveQueue           []*Update
	liveMutex           sync.RWMutex
}

const outBufferLength = 1000

// NewLocalSubscriber creates a new subscriber.
func NewLocalSubscriber(lastEventID string, logger Logger, topicSelectorStore *TopicSelectorStore) *LocalSubscriber {
	id := "urn:uuid:" + uuid.Must(uuid.NewV4()).String()
	s := &LocalSubscriber{
		Subscriber:          *NewSubscriber(logger, topicSelectorStore),
		responseLastEventID: make(chan string, 1),
		out:                 make(chan *Update, outBufferLength),
	}

	s.ID = id
	s.EscapedID = url.QueryEscape(id)
	s.RequestLastEventID = lastEventID

	return s
}

// Dispatch an update to the subscriber.
// Security checks must (topics matching) be done before calling Dispatch,
// for instance by calling Match.
func (s *LocalSubscriber) Dispatch(u *Update, fromHistory bool) bool {
	s.outMutex.Lock()

	if s.disconnected.Load() > 0 {
		s.outMutex.Unlock()
		return false
	}

	if !fromHistory && s.ready.Load() < 1 {
		s.liveMutex.Lock()
		defer s.liveMutex.Unlock()

		s.liveQueue = append(s.liveQueue, u)
		s.outMutex.Unlock()

		return true
	}

	select {
	case s.out <- u:
		s.outMutex.Unlock()
	default:
		s.handleFullChan()

		return false
	}

	return true
}

// Ready flips the ready flag to true and flushes queued live updates returning number of events flushed.
func (s *LocalSubscriber) Ready() (n int) {
	s.outMutex.Lock()

	s.liveMutex.RLock()
	defer s.liveMutex.RUnlock()

	if s.disconnected.Load() > 0 || s.ready.Load() > 0 {
		s.outMutex.Unlock()

		return 0
	}

	for _, u := range s.liveQueue {
		select {
		case s.out <- u:
			n++
		default:
			s.ready.Store(1)
			s.handleFullChan()
			s.liveQueue = nil

			return n
		}
	}

	s.ready.Store(1)
	s.liveQueue = nil

	s.outMutex.Unlock()

	return n
}

// Receive returns a chan when incoming updates are dispatched.
func (s *LocalSubscriber) Receive() <-chan *Update {
	return s.out
}

// HistoryDispatched must be called when all messages coming from the history have been dispatched.
func (s *LocalSubscriber) HistoryDispatched(responseLastEventID string) {
	s.responseLastEventID <- responseLastEventID
}

// Disconnect disconnects the subscriber.
func (s *LocalSubscriber) Disconnect() {
	s.outMutex.Lock()
	defer s.outMutex.Unlock()

	if s.disconnected.Load() > 0 {
		return // already disconnected
	}

	s.disconnected.Store(1)
	close(s.out)
}

// handleFullChan disconnects the subscriber when the out channel is full.
func (s *LocalSubscriber) handleFullChan() {
	defer s.outMutex.Unlock()
	if s.disconnected.Load() > 0 {
		return // already disconnected
	}

	s.disconnected.Store(1)
	close(s.out)

	if c := s.logger.Check(zap.ErrorLevel, "subscriber unable to receive updates fast enough"); c != nil {
		c.Write(zap.Object("subscriber", s))
	}
}
