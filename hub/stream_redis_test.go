package hub

import (
	"context"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func redisStreamConnect(t *testing.T, URL string) *RedisStream {
	url, _ := url.Parse(URL)
	stream, err := NewRedisStream(&Options{TransportURL: url})
	if err != nil {
		t.Skip("skipping test in short mode.")
	}
	require.Nil(t, err)
	stream.client.FlushAll()

	return stream
}

func TestRedisStreamHistory(t *testing.T) {
	stream := redisStreamConnect(t, "redis://localhost/0")
	defer stream.Close()
	assert.Implements(t, (*Stream)(nil), stream)

	for i := 1; i <= 10; i++ {
		stream.Write(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	pipe, err := stream.CreatePipe("8")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var count uint64
	go func() {
		for {
			_, err := pipe.Read(context.Background())
			if err == ErrClosedPipe {
				return
			}
			atomic.AddUint64(&count, 1)
		}
	}()

	// let time to the reading loop to process as many message as it can. Then we close the pipe
	time.Sleep(5 * time.Millisecond)
	pipe.Close()

	// the reading loop should have read the #9 and #10 messages
	assert.Equal(t, uint64(2), atomic.LoadUint64(&count))
}

func TestRedisStreamHistoryWithSizeLimited(t *testing.T) {
	stream := redisStreamConnect(t, "redis://localhost/0?size=5")
	defer stream.Close()

	for i := 1; i <= 10; i++ {
		stream.Write(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	pipe, err := stream.CreatePipe("8")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var count uint64
	go func() {
		for {
			_, err := pipe.Read(context.Background())
			if err == ErrClosedPipe {
				return
			}
			atomic.AddUint64(&count, 1)
		}
	}()

	// let time to the reading loop to process as many message as it can. Then we close the pipe
	time.Sleep(5 * time.Millisecond)
	pipe.Close()

	// the reading loop should have read the #9 and #10 messages
	assert.Equal(t, uint64(2), atomic.LoadUint64(&count))
}

func TestRedisStreamHistoryAndLive(t *testing.T) {
	stream := redisStreamConnect(t, "redis://localhost/0")
	defer stream.Close()

	for i := 1; i <= 10; i++ {
		stream.Write(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	pipe, err := stream.CreatePipe("8")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var count uint64
	go func() {
		for {
			_, err := pipe.Read(context.Background())
			if err == ErrClosedPipe {
				return
			}
			atomic.AddUint64(&count, 1)
		}
	}()
	stream.Write(&Update{Event: Event{ID: "1"}})

	// let time to the reading loop to process as many message as it can. Then we close the pipe
	time.Sleep(5 * time.Millisecond)
	pipe.Close()

	// the reading loop should have read the #9, #10 messages then the #11
	assert.Equal(t, uint64(3), atomic.LoadUint64(&count))
}

func TestNewRedisStream(t *testing.T) {
	url, _ := url.Parse("redis://localhost/0?stream_name=demo")
	stream, err := NewRedisStream(&Options{TransportURL: url})
	assert.Nil(t, err)
	require.NotNil(t, stream)
	stream.Close()

	url, _ = url.Parse("redis://localhost/0?master_name=demo")
	stream, err = NewRedisStream(&Options{TransportURL: url})
	assert.NotNil(t, err)
	assert.EqualError(t, err, `redis connection "redis://localhost/0": redis: all sentinels are unreachable`)

	url, _ = url.Parse("redis://localhost/azerty")
	_, err = NewRedisStream(&Options{TransportURL: url})
	assert.EqualError(t, err, `invalid redis "redis://localhost/azerty" dsn: invalid redis database number: "azerty"`)

	url, _ = url.Parse("redis://127.0.0.1:888/0")
	_, err = NewRedisStream(&Options{TransportURL: url})
	assert.EqualError(t, err, `redis connection "redis://127.0.0.1:888/0": dial tcp 127.0.0.1:888: connect: connection refused`)

	url, _ = url.Parse("redis://localhost/0?size=invalid")
	_, err = NewRedisStream(&Options{TransportURL: url})
	assert.EqualError(t, err, "invalid redis \"redis://localhost/0?size=invalid\" dsn: parameter size: strconv.ParseInt: parsing \"invalid\": invalid syntax")
}

func TestRedisStreamWriteIsNotDispatchedUntilListen(t *testing.T) {
	stream := redisStreamConnect(t, "redis://localhost/0")
	defer stream.Close()

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

func TestRedisStreamWriteIsDispatched(t *testing.T) {
	stream := redisStreamConnect(t, "redis://localhost/0")
	defer stream.Close()

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

func TestRedisStreamClosed(t *testing.T) {
	stream := redisStreamConnect(t, "redis://localhost/0")
	defer stream.Close()

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
