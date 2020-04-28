package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeReadWrite(t *testing.T) {
	var u *Update
	pipe := NewPipe()

	pipe.Write(u)

	update, ok := <-pipe.Read()
	assert.True(t, ok)
	assert.Equal(t, u, update)
}

func TestPipeReadClosed(t *testing.T) {
	pipe := NewPipe()

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
	pipe := NewPipe()

	assert.True(t, pipe.Write(u))

	pipe.Close()

	assert.False(t, pipe.Write(u))
}
