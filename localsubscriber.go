package mercure

import (
	"context"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/gofrs/uuid"
	"go.uber.org/zap"
)

// LocalSubscriber represents a client subscribed to a list of topics on the current hub.
type LocalSubscriber struct {
	Subscriber

	ctx                 context.Context
	cancel              context.CancelFunc
	out                 chan *Update
	responseLastEventID chan string
	ready               int32
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

	s.ctx, s.cancel = context.WithCancel(context.Background())

	return s
}

// Dispatch an update to the subscriber.
// Security checks must (topics matching) be done before calling Dispatch,
// for instance by calling Match.
func (s *LocalSubscriber) Dispatch(u *Update, fromHistory bool) bool {
	select {
	case <-s.ctx.Done():
		return false
	default:
	}

	if !fromHistory && atomic.LoadInt32(&s.ready) < 1 {
		s.liveMutex.Lock()
		if s.ready < 1 {
			s.liveQueue = append(s.liveQueue, u)
			s.liveMutex.Unlock()

			return true
		}
		s.liveMutex.Unlock()
	}

	select {
	case <-s.ctx.Done():
		return false
	default:
	}

	select {
	case <-s.ctx.Done():
		return false
	case s.out <- u:
	default:
		s.handleFullChan()

		return false
	}

	return true
}

// Ready flips the ready flag to true and flushes queued live updates returning number of events flushed.
func (s *LocalSubscriber) Ready() (n int) {
	s.liveMutex.Lock()

	for _, u := range s.liveQueue {
		select {
		case s.out <- u:
			n++
		default:
			s.handleFullChan()
			s.liveQueue = nil
			s.liveMutex.Unlock()

			return n
		}
	}
	atomic.StoreInt32(&s.ready, 1)
	s.liveQueue = nil

	s.liveMutex.Unlock()

	return n
}

// Receive returns a chan when incoming updates are dispatched.
func (s *LocalSubscriber) Receive() <-chan *Update {
	return s.out
}

func (s *LocalSubscriber) Disconnected() <-chan struct{} {
	return s.ctx.Done()
}

// HistoryDispatched must be called when all messages coming from the history have been dispatched.
func (s *LocalSubscriber) HistoryDispatched(responseLastEventID string) {
	s.responseLastEventID <- responseLastEventID
}

// Disconnect disconnects the subscriber.
func (s *LocalSubscriber) Disconnect() {
	s.cancel()
}

// handleFullChan disconnects the subscriber when the out channel is full.
func (s *LocalSubscriber) handleFullChan() {
	s.cancel()

	if c := s.logger.Check(zap.ErrorLevel, "subscriber unable to receive updates fast enough"); c != nil {
		c.Write(zap.Object("subscriber", s))
	}
}
