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
func (p *Pipe) Write(update *Update) bool {
	select {
	case <-p.done:
		return false
	default:
	}

	p.updates <- update

	return true
}

// Read returns the next unfetch update from the pipe with a context
func (p *Pipe) Read(ctx context.Context) (*Update, error) {
	// If you return new errors, don't forget to handle them in subscribe.go
	select {
	case <-p.done:
		return nil, ErrClosedPipe
	case <-ctx.Done():
		return nil, ctx.Err()
	case update := <-p.updates:
		return update, nil
	}
}

// IsClosed returns true if the pipe is closed
func (p *Pipe) IsClosed() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// Close closes the pipe
func (p *Pipe) Close() {
	select {
	case <-p.done:
		// Already closed. Don't close again.
	default:
		close(p.done)
	}
}
