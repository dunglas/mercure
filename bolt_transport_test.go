package mercure

import (
	"bytes"
	"encoding/binary"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

func createBoltTransport(dsn string) *BoltTransport {
	u, _ := url.Parse(dsn)
	transport, _ := NewBoltTransport(u, zap.NewNop())

	return transport.(*BoltTransport)
}

func TestBoltTransportHistory(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		transport.Dispatch(&Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: topics,
		})
	}

	s := NewSubscriber("8", transport.logger, NewTopicSelectorStore())
	s.Topics = topics
	go s.start()

	require.Nil(t, transport.AddSubscriber(s))

	var count int
	for {
		u := <-s.Receive()
		// the reading loop must read the #9 and #10 messages
		assert.Equal(t, strconv.Itoa(9+count), u.ID)
		count++
		if count == 2 {
			return
		}
	}
}

func TestBoltTransportRetrieveAllHistory(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		transport.Dispatch(&Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: topics,
		})
	}

	s := NewSubscriber(EarliestLastEventID, transport.logger, NewTopicSelectorStore())
	s.Topics = topics
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	var count int
	for {
		u := <-s.Receive()
		// the reading loop must read all messages
		count++
		assert.Equal(t, strconv.Itoa(count), u.ID)
		if count == 10 {
			return
		}
	}
}

func TestBoltTransportHistoryAndLive(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		transport.Dispatch(&Update{
			Topics: topics,
			Event:  Event{ID: strconv.Itoa(i)},
		})
	}

	s := NewSubscriber("8", transport.logger, NewTopicSelectorStore())
	s.Topics = topics
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var count int
		for {
			u := <-s.Receive()

			// the reading loop must read the #9, #10 and #11 messages
			assert.Equal(t, strconv.Itoa(9+count), u.ID)
			count++
			if count == 3 {
				return
			}
		}
	}()

	transport.Dispatch(&Update{
		Event:  Event{ID: "11"},
		Topics: topics,
	})

	wg.Wait()
}

func TestBoltTransportPurgeHistory(t *testing.T) {
	transport := createBoltTransport("bolt://test.db?size=5&cleanup_frequency=1")
	defer transport.Close()
	defer os.Remove("test.db")

	for i := 0; i < 12; i++ {
		transport.Dispatch(&Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: []string{"https://example.com/foo"},
		})
	}

	transport.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("updates"))

		assert.Equal(t, 5, b.Stats().KeyN)

		return nil
	})
}

func TestNewBoltTransport(t *testing.T) {
	u, _ := url.Parse("bolt://test.db?bucket_name=demo")
	transport, err := NewBoltTransport(u, zap.NewNop())
	assert.Nil(t, err)
	require.NotNil(t, transport)
	transport.Close()

	u, _ = url.Parse("bolt://")
	_, err = NewBoltTransport(u, zap.NewNop())
	assert.EqualError(t, err, `"bolt:": invalid transport: missing path`)

	u, _ = url.Parse("bolt:///test.db")
	_, err = NewBoltTransport(u, zap.NewNop())

	// The exact error message depends of the OS
	assert.Contains(t, err.Error(), "open /test.db:")

	u, _ = url.Parse("bolt://test.db?cleanup_frequency=invalid")
	_, err = NewBoltTransport(u, zap.NewNop())
	assert.EqualError(t, err, `"bolt://test.db?cleanup_frequency=invalid": invalid "cleanup_frequency" parameter "invalid": invalid transport: strconv.ParseFloat: parsing "invalid": invalid syntax`)

	u, _ = url.Parse("bolt://test.db?size=invalid")
	_, err = NewBoltTransport(u, zap.NewNop())
	assert.EqualError(t, err, `"bolt://test.db?size=invalid": invalid "size" parameter "invalid": invalid transport: strconv.ParseUint: parsing "invalid": invalid syntax`)
}

func TestBoltTransportDoNotDispatchedUntilListen(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", transport.logger, NewTopicSelectorStore())
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	var (
		readUpdate *Update
		ok         bool
		wg         sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		select {
		case readUpdate = <-s.Receive():
		case <-s.disconnected:
			ok = true
		}

		wg.Done()
	}()

	s.Disconnect()

	wg.Wait()
	assert.Nil(t, readUpdate)
	assert.True(t, ok)
}

func TestBoltTransportDispatch(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", transport.logger, NewTopicSelectorStore())
	s.Topics = []string{"https://example.com/foo"}
	go s.start()

	require.Nil(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.Topics}
	require.Nil(t, transport.Dispatch(u))
	assert.Equal(t, u, <-s.Receive())
}

func TestBoltTransportClosed(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", transport.logger, NewTopicSelectorStore())
	s.Topics = []string{"https://example.com/foo"}
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	require.Nil(t, transport.Close())
	require.NotNil(t, transport.AddSubscriber(s))

	assert.Equal(t, transport.Dispatch(&Update{Topics: s.Topics}), ErrClosedTransport)

	_, ok := <-s.disconnected
	assert.False(t, ok)
}

func TestBoltCleanDisconnectedSubscribers(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")

	tss := NewTopicSelectorStore()

	s1 := NewSubscriber("", transport.logger, tss)
	go s1.start()
	require.Nil(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", transport.logger, tss)
	go s2.start()
	require.Nil(t, transport.AddSubscriber(s2))

	assert.Len(t, transport.subscribers, 2)

	s1.Disconnect()
	assert.Len(t, transport.subscribers, 2)

	transport.Dispatch(&Update{Topics: s1.Topics})
	assert.Len(t, transport.subscribers, 1)

	s2.Disconnect()
	assert.Len(t, transport.subscribers, 1)

	transport.Dispatch(&Update{})
	assert.Len(t, transport.subscribers, 0)
}

func TestBoltGetSubscribers(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")

	tss := NewTopicSelectorStore()

	s1 := NewSubscriber("", transport.logger, tss)
	go s1.start()
	require.Nil(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", transport.logger, tss)
	go s2.start()
	require.Nil(t, transport.AddSubscriber(s2))

	lastEventID, subscribers := transport.GetSubscribers()
	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, s1)
	assert.Contains(t, subscribers, s2)
}

func TestBoltLastEventID(t *testing.T) {
	db, err := bolt.Open("test.db", 0o600, nil)
	defer os.Remove("test.db")
	require.Nil(t, err)

	db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(defaultBoltBucketName))
		require.Nil(t, err)

		seq, err := bucket.NextSequence()
		require.Nil(t, err)

		prefix := make([]byte, 8)
		binary.BigEndian.PutUint64(prefix, seq)

		// The sequence value is prepended to the update id to create an ordered list
		key := bytes.Join([][]byte{prefix, []byte("foo")}, []byte{})

		// The DB is append only
		bucket.FillPercent = 1

		return bucket.Put(key, []byte("invalid"))
	})
	require.Nil(t, db.Close())

	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()

	lastEventID, _ := transport.GetSubscribers()
	assert.Equal(t, "foo", lastEventID)
}
