package hub

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
)

// ErrClosedPipe is returned by the Pipe's Write and Read methods after a call to Close.
var ErrClosedPipe = errors.New("hub: read/write on closed Pipe")

// Pipe convey Update to reader in a closable chan.
type Pipe struct {
	updates chan *Update
	done    chan struct{}
}

// NewPipe creates pipes.
func NewPipe() *Pipe {
	return &Pipe{
		make(chan *Update, 5),
		make(chan struct{}),
	}
}

// Write pushes updates in the pipe. Returns true is the update is pushed, false otherwise.
func (p *Pipe) Write(update *Update) bool {
	select {
	case <-p.done:
		return false
	default:
	}

	// The updates channel is buffered, if the buffer is full and it blocks for too long we close it
	select {
	case p.updates <- update:
		return true
	case <-time.After(1 * time.Second):
		close(p.updates)
		log.Info("Messages blocked, pipe closed.")
		return false
	}
}

// Read returns a channel containing updates.
func (p *Pipe) Read() chan *Update {
	return p.updates
}

// IsClosed returns true if the pipe is closed.
func (p *Pipe) IsClosed() bool {
	select {
	case <-p.done:
		return true
	default:
	}

	return false
}

// Close closes the pipe.
func (p *Pipe) Close() {
	select {
	case <-p.done:
		// Already closed. Don't close again.
	default:
	}

	close(p.done)
}
