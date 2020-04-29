package hub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipeReadWrite(t *testing.T) {
	var u *Update
	pipe := NewPipe(5, time.Second)

	pipe.Write(u)

	update, ok := <-pipe.Read()
	assert.True(t, ok)
	assert.Equal(t, u, update)
}

func TestPipeReadClosed(t *testing.T) {
	pipe := NewPipe(5, time.Second)

	assert.False(t, pipe.IsClosed())
	pipe.Close()

	assert.True(t, pipe.IsClosed())

	close(pipe.Read())
	update, ok := <-pipe.Read()
	assert.Nil(t, update)
	assert.False(t, ok)
}

func TestPipeWriteClosed(t *testing.T) {
	var u *Update
	pipe := NewPipe(5, time.Second)

	assert.True(t, pipe.Write(u))

	pipe.Close()

	assert.False(t, pipe.Write(u))
}
