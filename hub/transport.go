package hub

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Transport provides methods to read and write updates.
type Transport interface {
	// Write pushes updates in the Transport.
	Write(update *Update) error

	// CreatePipe returns a pipe fetching updates from the given point in time.
	CreatePipe(fromID string) (*Pipe, error)

	// Close closes the Transport.
	Close() error
}

// ErrClosedTransport is returned by the Transport's Write and CreatePipe methods after a call to Close.
var ErrClosedTransport = errors.New("hub: read/write on closed Transport")

// NewTransport create a transport using the backend matching the given TransportURL.
func NewTransport(config *viper.Viper) (Transport, error) {
	bs := config.GetInt("update_buffer_size")
	bt := config.GetDuration("update_buffer_full_timeout")
	tu := config.GetString("transport_url")
	if tu == "" {
		return NewLocalTransport(bs, bt), nil
	}

	u, err := url.Parse(tu)
	if err != nil {
		return nil, fmt.Errorf("transport_url: %w", err)
	}

	switch u.Scheme {
	case "null":
		return NewLocalTransport(bs, bt), nil

	case "bolt":
		return NewBoltTransport(u, bs, bt)
	}

	return nil, fmt.Errorf(`no Transport available for "%s"`, tu)
}

// LocalTransport implements the TransportInterface without database and simply broadcast the live Updates.
type LocalTransport struct {
	sync.RWMutex
	pipes             map[*Pipe]struct{}
	done              chan struct{}
	bufferSize        int
	bufferFullTimeout time.Duration
}

// NewLocalTransport create a new LocalTransport.
func NewLocalTransport(bufferSize int, bufferFullTimeout time.Duration) *LocalTransport {
	return &LocalTransport{
		pipes:             make(map[*Pipe]struct{}),
		done:              make(chan struct{}),
		bufferSize:        bufferSize,
		bufferFullTimeout: bufferFullTimeout,
	}
}

// Write pushes updates in the Transport.
func (t *LocalTransport) Write(update *Update) error {
	select {
	case <-t.done:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	defer t.Unlock()
	for pipe := range t.pipes {
		if !pipe.Write(update) {
			delete(t.pipes, pipe)
		}
	}

	return nil
}

// CreatePipe returns a pipe fetching updates from the given point in time.
func (t *LocalTransport) CreatePipe(fromID string) (*Pipe, error) {
	t.Lock()
	defer t.Unlock()

	select {
	case <-t.done:
		return nil, ErrClosedTransport
	default:
	}

	pipe := NewPipe(t.bufferSize, t.bufferFullTimeout)
	t.pipes[pipe] = struct{}{}

	return pipe, nil
}

// Close closes the Transport.
func (t *LocalTransport) Close() error {
	select {
	case <-t.done:
		return nil
	default:
	}

	t.RLock()
	defer t.RUnlock()
	for pipe := range t.pipes {
		close(pipe.Read())
	}
	close(t.done)

	return nil
}
