package hub

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestBoltTransportHistory(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")

	for i := 1; i <= 10; i++ {
		transport.Write(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	pipe, err := transport.CreatePipe("8")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var wg sync.WaitGroup
	go func() {
		var count int
		for {
			u, err := pipe.Read(context.Background())
			if err == ErrClosedPipe {
				return
			}

			// the reading loop must read the #9 and #10 messages
			assert.Equal(t, strconv.Itoa(9+count), u.ID)
			count++
			if count == 2 {
				wg.Done()
				return
			}
		}
	}()

	wg.Wait()
	pipe.Close()
}

func TestBoltTransportHistoryAndLive(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")

	for i := 1; i <= 10; i++ {
		transport.Write(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	pipe, err := transport.CreatePipe("8")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		var count int
		for {
			u, err := pipe.Read(context.Background())
			if err == ErrClosedPipe {
				return
			}

			// the reading loop must read the #9, #10 and #11 messages
			assert.Equal(t, strconv.Itoa(9+count), u.ID)
			count++
			if count == 3 {
				wg.Done()
				return
			}
		}
	}()

	transport.Write(&Update{Event: Event{ID: "11"}})

	wg.Wait()
	pipe.Close()
}

func TestBoltTransportPurgeHistory(t *testing.T) {
	u, _ := url.Parse("bolt://test.db?size=5&cleanup_frequency=1")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")

	for i := 0; i < 12; i++ {
		transport.Write(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	transport.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("updates"))

		assert.Equal(t, 5, b.Stats().KeyN)

		return nil
	})
}

func TestNewBoltTransport(t *testing.T) {
	u, _ := url.Parse("bolt://test.db?bucket_name=demo")
	transport, err := NewBoltTransport(u)
	assert.Nil(t, err)
	require.NotNil(t, transport)
	transport.Close()

	u, _ = url.Parse("bolt://")
	_, err = NewBoltTransport(u)
	assert.EqualError(t, err, `invalid bolt DSN "bolt:": missing path`)

	u, _ = url.Parse("bolt:///test.db")
	_, err = NewBoltTransport(u)

	// The exact error message depends of the OS
	assert.Contains(t, err.Error(), `invalid bolt DSN "bolt:///test.db": open /test.db: `)

	u, _ = url.Parse("bolt://test.db?cleanup_frequency=invalid")
	_, err = NewBoltTransport(u)
	assert.EqualError(t, err, `invalid bolt "bolt://test.db?cleanup_frequency=invalid" dsn: parameter cleanup_frequency: strconv.ParseFloat: parsing "invalid": invalid syntax`)

	u, _ = url.Parse("bolt://test.db?size=invalid")
	_, err = NewBoltTransport(u)
	assert.EqualError(t, err, `invalid bolt "bolt://test.db?size=invalid" dsn: parameter size: strconv.ParseUint: parsing "invalid": invalid syntax`)
}

func TestBoltTransportWriteIsNotDispatchedUntilListen(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	pipe, err := transport.CreatePipe("")
	assert.Nil(t, err)
	require.NotNil(t, pipe)

	var (
		readUpdate *Update
		readError  error
		m          sync.Mutex
		wg         sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		m.Lock()
		defer m.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		go wg.Done()
		readUpdate, readError = pipe.Read(ctx)
	}()

	wg.Wait()
	pipe.Close()

	m.Lock()
	defer m.Unlock()
	assert.Nil(t, readUpdate)
	assert.Equal(t, ErrClosedPipe, readError)
}

func TestBoltTransportWriteIsDispatched(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	pipe, err := transport.CreatePipe("")
	assert.Nil(t, err)
	require.NotNil(t, pipe)
	defer pipe.Close()

	var (
		readUpdate *Update
		readError  error
		m          sync.Mutex
		wg         sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		m.Lock()
		defer m.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		go wg.Done()
		readUpdate, readError = pipe.Read(ctx)
	}()

	wg.Wait()
	err = transport.Write(&Update{})
	assert.Nil(t, err)

	m.Lock()
	defer m.Unlock()

	assert.Nil(t, readError)
	assert.NotNil(t, readUpdate)
}

func TestBoltTransportClosed(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	pipe, _ := transport.CreatePipe("")
	require.NotNil(t, pipe)

	err := transport.Close()
	assert.Nil(t, err)

	_, err = transport.CreatePipe("")
	assert.Equal(t, err, ErrClosedTransport)

	err = transport.Write(&Update{})
	assert.Equal(t, err, ErrClosedTransport)

	_, err = pipe.Read(context.Background())
	assert.Equal(t, err, ErrClosedPipe)
}

func TestBoltCleanClosedPipes(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")

	pipe, _ := transport.CreatePipe("")
	require.NotNil(t, pipe)

	assert.Len(t, transport.pipes, 1)

	pipe.Close()
	assert.Len(t, transport.pipes, 1)

	transport.Write(&Update{})
	assert.Len(t, transport.pipes, 0)
}
