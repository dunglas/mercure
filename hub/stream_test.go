package hub

import (
	"context"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveStreamWriteIsNotDispatchedUntilListen(t *testing.T) {
	stream := NewLiveStream()
	defer stream.Close()
	assert.Implements(t, (*Stream)(nil), stream)

	err := stream.Write(&Update{})
	assert.Nil(t, err)

	pipe, err := stream.CreatePipe("")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var (
		readUpdate *Update
		readError  error
		m          sync.Mutex
	)
	go func() {
		m.Lock()
		defer m.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		readUpdate, readError = pipe.Read(ctx)
	}()

	// let time to the goroutine to start listening before closing the pipe
	time.Sleep(5 * time.Millisecond)
	pipe.Close()

	m.Lock()
	defer m.Unlock()
	assert.Nil(t, readUpdate)
	assert.Equal(t, ErrClosedPipe, readError)
}

func TestLiveStreamWriteIsDispatched(t *testing.T) {
	stream := NewLiveStream()
	defer stream.Close()
	assert.Implements(t, (*Stream)(nil), stream)

	pipe, err := stream.CreatePipe("")
	assert.Nil(t, err)
	require.NotNil(t, pipe)
	defer pipe.Close()

	var (
		readUpdate *Update
		readError  error
		m          sync.Mutex
	)
	go func() {
		m.Lock()
		defer m.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		readUpdate, readError = pipe.Read(ctx)
	}()

	// let time to the goroutine to start listening before sending the first message
	time.Sleep(5 * time.Millisecond)
	err = stream.Write(&Update{})
	assert.Nil(t, err)

	m.Lock()
	defer m.Unlock()

	assert.Nil(t, readError)
	assert.NotNil(t, readUpdate)
}

func TestLiveStreamClosed(t *testing.T) {
	stream := NewLiveStream()
	defer stream.Close()
	assert.Implements(t, (*Stream)(nil), stream)

	pipe, _ := stream.CreatePipe("")
	require.NotNil(t, pipe)

	err := stream.Close()
	assert.Nil(t, err)

	_, err = stream.CreatePipe("")
	assert.Equal(t, err, ErrClosedStream)

	err = stream.Write(&Update{})
	assert.Equal(t, err, ErrClosedStream)

	_, err = pipe.Read(context.Background())
	assert.Equal(t, err, ErrClosedPipe)
}

func TestLiveCleanClosedPipes(t *testing.T) {
	stream := NewLiveStream()
	defer stream.Close()

	pipe, _ := stream.CreatePipe("")
	require.NotNil(t, pipe)

	assert.Len(t, stream.pipes, 1)

	pipe.Close()
	assert.Len(t, stream.pipes, 1)

	stream.Write(&Update{})
	assert.Len(t, stream.pipes, 0)
}

func TestLivePipeReadingBlocks(t *testing.T) {
	stream := NewLiveStream()
	defer stream.Close()
	assert.Implements(t, (*Stream)(nil), stream)

	pipe, err := stream.CreatePipe("")
	assert.Nil(t, err)
	require.NotNil(t, pipe)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Wait()
		err := stream.Write(&Update{})
		assert.Nil(t, err)
	}()

	wg.Done()
	u, err := pipe.Read(context.Background())
	assert.Nil(t, err)
	assert.NotNil(t, u)
}

func TestNewStream(t *testing.T) {
	stream, err := NewStream(&Options{TransportURL: nil})
	assert.Nil(t, err)
	require.NotNil(t, stream)
	stream.Close()
	assert.IsType(t, &LiveStream{}, stream)

	url, _ := url.Parse("bolt://test.db")
	stream, _ = NewStream(&Options{TransportURL: url})
	assert.Nil(t, err)
	require.NotNil(t, stream)
	stream.Close()
	os.Remove("test.db")
	assert.IsType(t, &BoltStream{}, stream)

	url, _ = url.Parse("redis://localhost/0")
	stream, _ = NewStream(&Options{TransportURL: url})
	assert.Nil(t, err)
	require.NotNil(t, stream)
	stream.Close()
	os.Remove("test.db")
	assert.IsType(t, &RedisStream{}, stream)

	url, _ = url.Parse("nothing:")
	stream, err = NewStream(&Options{TransportURL: url})
	assert.Nil(t, stream)
	assert.NotNil(t, err)
	assert.EqualError(t, err, `no stream available for "nothing:"`)
}
