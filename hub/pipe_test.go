package hub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipeReadWrite(t *testing.T) {
	var u *Update
	pipe := NewPipe()

	pipe.Write(u)

	update, err := pipe.Read(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, u, update)
}

func TestPipeReadClosed(t *testing.T) {
	pipe := NewPipe()

	assert.False(t, pipe.IsClosed())
	pipe.Close()

	assert.True(t, pipe.IsClosed())

	update, err := pipe.Read(context.Background())
	assert.Nil(t, update)
	assert.Equal(t, ErrClosedPipe, err)
}

func TestPipeReadWithContext(t *testing.T) {
	pipe := NewPipe()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	update, err := pipe.Read(ctx)
	assert.Nil(t, update)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestPipeWriteClosed(t *testing.T) {
	var u *Update
	pipe := NewPipe()

	assert.True(t, pipe.Write(u))

	pipe.Close()

	assert.False(t, pipe.Write(u))
}
