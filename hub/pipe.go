package hub

import (
	"context"
	"errors"
)

// ErrClosedPipe is returned by the Pipe's Write and Read methods after a call to Close.
var ErrClosedPipe = errors.New("hub: read/write on closed Pipe")

// Pipe convey Update to reader in a closable chan
type Pipe struct {
	updates chan *Update
	done    chan struct{}
}

// NewPipe creates pipes
func NewPipe() *Pipe {
	return &Pipe{make(chan *Update, 1), make(chan struct{})}
}

// Write pushes updates in the pipe. Returns true is the update is pushed, false otherwise.
func (c *Pipe) Write(update *Update) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	c.updates <- update

	return true
}

// Read returns the next unfetch update from the pipe with a context
func (c *Pipe) Read(ctx context.Context) (*Update, error) {
	select {
	case <-c.done:
		return nil, ErrClosedPipe
	case <-ctx.Done():
		return nil, ctx.Err()
	case update := <-c.updates:
		return update, nil
	}
}

// Close closes the pipe
func (c *Pipe) Close() {
	select {
	case <-c.done:
		// Already closed. Don't close again.
	default:
		close(c.done)
	}
}
