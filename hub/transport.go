package hub

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/spf13/viper"
)

// Transport provides methods to read and write updates
type Transport interface {
	// Write pushes updates in the Transport
	Write(update *Update) error

	// CreatePipe returns a pipe fetching updates from the given point in time
	CreatePipe(fromID string) (*Pipe, error)

	// Close closes the Transport
	Close() error
}

// ErrClosedTransport is returned by the Transport's Write and CreatePipe methods after a call to Close.
var ErrClosedTransport = errors.New("hub: read/write on closed Transport")

// NewTransport create a transport using the backend matching the given TransportURL
func NewTransport(config *viper.Viper) (Transport, error) {
	tu := config.GetString("transport_url")
	if tu == "" {
		return NewLocalTransport(), nil
	}

	u, err := url.Parse(tu)
	if err != nil {
		return nil, fmt.Errorf("transport_url: %w", err)
	}

	switch u.Scheme {
	case "null":
		return NewLocalTransport(), nil

	case "bolt":
		return NewBoltTransport(u)
	}

	return nil, fmt.Errorf(`no Transport available for "%s"`, tu)
}

// LocalTransport implements the TransportInterface without database and simply broadcast the live Updates
type LocalTransport struct {
	sync.RWMutex
	pipes map[*Pipe]struct{}
	done  chan struct{}
}

// NewLocalTransport create a new LocalTransport
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{pipes: make(map[*Pipe]struct{}), done: make(chan struct{})}
}

// Write pushes updates in the Transport
func (t *LocalTransport) Write(update *Update) error {
	select {
	case <-t.done:
		return ErrClosedTransport
	default:
	}

	var (
		err         error
		closedPipes []*Pipe
	)

	t.RLock()

	for pipe := range t.pipes {
		if !pipe.Write(update) {
			closedPipes = append(closedPipes, pipe)
		}
	}

	t.RUnlock()
	t.Lock()

	for _, pipe := range closedPipes {
		delete(t.pipes, pipe)
	}

	t.Unlock()

	return err
}

// CreatePipe returns a pipe fetching updates from the given point in time
func (t *LocalTransport) CreatePipe(fromID string) (*Pipe, error) {
	t.Lock()
	defer t.Unlock()

	select {
	case <-t.done:
		return nil, ErrClosedTransport
	default:
	}

	pipe := NewPipe()
	t.pipes[pipe] = struct{}{}

	return pipe, nil
}

// Close closes the Transport
func (t *LocalTransport) Close() error {
	select {
	case <-t.done:
		// Already closed. Don't close again.
	default:
		t.RLock()
		defer t.RUnlock()
		for pipe := range t.pipes {
			pipe.Close()
		}
		close(t.done)
	}

	return nil
}
