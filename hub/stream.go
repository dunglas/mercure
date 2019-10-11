package hub

import (
	"errors"
	"fmt"
	"sync"
)

// Stream provides methods to read and write updates
type Stream interface {
	// Write pushes updates in the Stream
	Write(update *Update) error

	// CreatePipe returns a pipe fetching updates from the given point in time
	CreatePipe(fromID string) (*Pipe, error)

	// Close closes the Stream
	Close() error
}

// ErrClosedStream is returned by the Stream's Write and CreatePipe methods after a call to Close.
var ErrClosedStream = errors.New("hub: read/write on closed Stream")

// NewStream create a stream using the backend matching the given TransportURL
func NewStream(options *Options) (Stream, error) {
	if options.TransportURL == nil || options.TransportURL.Scheme == "null" {
		return NewLiveStream(), nil
	}

	if options.TransportURL.Scheme == "bolt" {
		return NewBoltStream(options)
	}

	if options.TransportURL.Scheme == "redis" {
		return NewRedisStream(options)
	}

	return nil, fmt.Errorf(`no stream available for "%s"`, options.TransportURL)
}

// LiveStream implements the StreamInterface without database and simply broadcast the live Updates
type LiveStream struct {
	sync.RWMutex
	pipes map[*Pipe]struct{}
	done  chan struct{}
}

// NewLiveStream create a new LiveStream
func NewLiveStream() *LiveStream {
	return &LiveStream{pipes: make(map[*Pipe]struct{}), done: make(chan struct{})}
}

// Write pushes updates in the Stream
func (s *LiveStream) Write(update *Update) error {
	select {
	case <-s.done:
		return ErrClosedStream
	default:
	}

	var (
		err         error
		closedPipes []*Pipe
	)

	s.RLock()

	for pipe := range s.pipes {
		if !pipe.Write(update) {
			closedPipes = append(closedPipes, pipe)
		}
	}

	s.RUnlock()
	s.Lock()

	for _, pipe := range closedPipes {
		delete(s.pipes, pipe)
	}

	s.Unlock()

	return err
}

// CreatePipe returns a pipe fetching updates from the given point in time
func (s *LiveStream) CreatePipe(fromID string) (*Pipe, error) {
	s.Lock()
	defer s.Unlock()

	select {
	case <-s.done:
		return nil, ErrClosedStream
	default:
	}

	pipe := NewPipe()
	s.pipes[pipe] = struct{}{}

	return pipe, nil
}

// Close closes the Stream
func (s *LiveStream) Close() error {
	select {
	case <-s.done:
		// Already closed. Don't close again.
	default:
		s.RLock()
		defer s.RUnlock()
		for pipe := range s.pipes {
			pipe.Close()
		}
		close(s.done)
	}

	return nil
}
