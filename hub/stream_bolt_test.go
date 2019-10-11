package hub

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestBoltStreamHistory(t *testing.T) {
	url, _ := url.Parse("bolt://test.db")
	stream, _ := NewBoltStream(&Options{TransportURL: url})
	defer stream.Close()
	defer os.Remove("test.db")

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
	time.Sleep(10 * time.Millisecond)
	pipe.Close()

	// the reading loop should have read the #9 and #10 messages
	assert.Equal(t, uint64(2), atomic.LoadUint64(&count))
}

func TestBoltStreamHistoryAndLive(t *testing.T) {
	url, _ := url.Parse("bolt://test.db")
	stream, _ := NewBoltStream(&Options{TransportURL: url})
	defer stream.Close()
	defer os.Remove("test.db")

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

	stream.Write(&Update{Event: Event{ID: "11"}})

	// let time to the reading loop to process as many message as it can. Then we close the pipe
	time.Sleep(10 * time.Millisecond)
	pipe.Close()

	// the reading loop should have read the #9, #10 messages then the #11
	assert.Equal(t, uint64(3), atomic.LoadUint64(&count))
}

func TestBoltStreamPurgeHistory(t *testing.T) {
	url, _ := url.Parse("bolt://test.db?size=5&cleanup_frequency=1")
	stream, _ := NewBoltStream(&Options{TransportURL: url})
	defer stream.Close()
	defer os.Remove("test.db")

	for i := 0; i < 12; i++ {
		stream.Write(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	stream.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("updates"))

		assert.Equal(t, 5, b.Stats().KeyN)

		return nil
	})
}

func TestNewBoltStream(t *testing.T) {
	url, _ := url.Parse("bolt://test.db?bucket_name=demo")
	stream, err := NewBoltStream(&Options{TransportURL: url})
	assert.Nil(t, err)
	require.NotNil(t, stream)
	stream.Close()

	url, _ = url.Parse("bolt://")
	_, err = NewBoltStream(&Options{TransportURL: url})
	assert.EqualError(t, err, `invalid bolt "bolt:" dsn: missing path`)

	url, _ = url.Parse("bolt:///root/test.db")
	_, err = NewBoltStream(&Options{TransportURL: url})
	assert.EqualError(t, err, `invalid bolt "bolt:///root/test.db" dsn: open /root/test.db: permission denied`)

	url, _ = url.Parse("bolt://test.db?cleanup_frequency=invalid")
	_, err = NewBoltStream(&Options{TransportURL: url})
	assert.EqualError(t, err, `invalid bolt "bolt://test.db?cleanup_frequency=invalid" dsn: parameter cleanup_frequency: strconv.ParseFloat: parsing "invalid": invalid syntax`)

	url, _ = url.Parse("bolt://test.db?size=invalid")
	_, err = NewBoltStream(&Options{TransportURL: url})
	assert.EqualError(t, err, `invalid bolt "bolt://test.db?size=invalid" dsn: parameter size: strconv.ParseUint: parsing "invalid": invalid syntax`)
}

func TestBoltStreamWriteIsNotDispatchedUntilListen(t *testing.T) {
	url, _ := url.Parse("bolt://test.db")
	stream, _ := NewBoltStream(&Options{TransportURL: url})
	defer stream.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Stream)(nil), stream)

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

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		readUpdate, readError = pipe.Read(ctx)
	}()

	// let time to the goroutine to start listening before closing the pipe
	time.Sleep(10 * time.Millisecond)
	pipe.Close()

	m.Lock()
	defer m.Unlock()
	assert.Nil(t, readUpdate)
	assert.Equal(t, ErrClosedPipe, readError)
}

func TestBoltStreamWriteIsDispatched(t *testing.T) {
	url, _ := url.Parse("bolt://test.db")
	stream, _ := NewBoltStream(&Options{TransportURL: url})
	defer stream.Close()
	defer os.Remove("test.db")
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

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		readUpdate, readError = pipe.Read(ctx)
	}()

	// let time to the goroutine to start listening before sending the first message
	time.Sleep(10 * time.Millisecond)
	err = stream.Write(&Update{})
	assert.Nil(t, err)

	m.Lock()
	defer m.Unlock()

	assert.Nil(t, readError)
	assert.NotNil(t, readUpdate)
}

func TestBoltStreamClosed(t *testing.T) {
	url, _ := url.Parse("bolt://test.db")
	stream, _ := NewBoltStream(&Options{TransportURL: url})
	require.NotNil(t, stream)
	defer stream.Close()
	defer os.Remove("test.db")
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

func TestBoltCleanClosedPipes(t *testing.T) {
	url, _ := url.Parse("bolt://test.db")
	stream, _ := NewBoltStream(&Options{TransportURL: url})
	require.NotNil(t, stream)
	defer stream.Close()
	defer os.Remove("test.db")

	pipe, _ := stream.CreatePipe("")
	require.NotNil(t, pipe)

	assert.Len(t, stream.pipes, 1)

	pipe.Close()
	assert.Len(t, stream.pipes, 1)

	stream.Write(&Update{})
	assert.Len(t, stream.pipes, 0)
}
