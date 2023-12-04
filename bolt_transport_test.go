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

	s := NewSubscriber("8", transport.logger)
	s.SetTopics(topics, nil)

	require.NoError(t, transport.AddSubscriber(s))

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

func TestBoltTransportLogsBogusLastEventID(t *testing.T) {
	sink, logger := newTestLogger(t)
	defer sink.Reset()

	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u, logger)
	defer transport.Close()
	defer os.Remove("test.db")

	// make sure the db is not empty
	topics := []string{"https://example.com/foo"}
	transport.Dispatch(&Update{
		Event:  Event{ID: "1"},
		Topics: topics,
	})

	s := NewSubscriber("711131", logger)
	s.SetTopics(topics, nil)

	require.NoError(t, transport.AddSubscriber(s))

	log := sink.String()
	assert.Contains(t, log, `"LastEventID":"711131"`)
}

func TestBoltTopicSelectorHistory(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")

	transport.Dispatch(&Update{Topics: []string{"http://example.com/subscribed"}, Event: Event{ID: "1"}})
	transport.Dispatch(&Update{Topics: []string{"http://example.com/not-subscribed"}, Event: Event{ID: "2"}})
	transport.Dispatch(&Update{Topics: []string{"http://example.com/subscribed-public-only"}, Private: true, Event: Event{ID: "3"}})
	transport.Dispatch(&Update{Topics: []string{"http://example.com/subscribed-public-only"}, Event: Event{ID: "4"}})

	s := NewSubscriber(EarliestLastEventID, transport.logger)
	s.SetTopics([]string{"http://example.com/subscribed", "http://example.com/subscribed-public-only"}, []string{"http://example.com/subscribed"})

	require.NoError(t, transport.AddSubscriber(s))

	assert.Equal(t, "1", (<-s.Receive()).ID)
	assert.Equal(t, "4", (<-s.Receive()).ID)
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

	s := NewSubscriber(EarliestLastEventID, transport.logger)
	s.SetTopics(topics, nil)
	require.NoError(t, transport.AddSubscriber(s))

	var count int
	for {
		u := <-s.Receive()
		// the reading loop must read all messages
		count++
		assert.Equal(t, strconv.Itoa(count), u.ID)
		if count == 10 {
			break
		}
	}
	assert.Equal(t, 10, count)
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

	s := NewSubscriber("8", transport.logger)
	s.SetTopics(topics, nil)
	require.NoError(t, transport.AddSubscriber(s))

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
	require.NoError(t, err)
	require.NotNil(t, transport)
	transport.Close()

	u, _ = url.Parse("bolt://")
	_, err = NewBoltTransport(u, zap.NewNop())
	require.EqualError(t, err, `"bolt:": invalid transport: missing path`)

	u, _ = url.Parse("bolt:///test.db")
	_, err = NewBoltTransport(u, zap.NewNop())

	// The exact error message depends of the OS
	assert.Contains(t, err.Error(), "open /test.db:")

	u, _ = url.Parse("bolt://test.db?cleanup_frequency=invalid")
	_, err = NewBoltTransport(u, zap.NewNop())
	require.EqualError(t, err, `"bolt://test.db?cleanup_frequency=invalid": invalid "cleanup_frequency" parameter "invalid": invalid transport: strconv.ParseFloat: parsing "invalid": invalid syntax`)

	u, _ = url.Parse("bolt://test.db?size=invalid")
	_, err = NewBoltTransport(u, zap.NewNop())
	require.EqualError(t, err, `"bolt://test.db?size=invalid": invalid "size" parameter "invalid": invalid transport: strconv.ParseUint: parsing "invalid": invalid syntax`)
}

func TestBoltTransportDoNotDispatchUntilListen(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", transport.logger)
	require.NoError(t, transport.AddSubscriber(s))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for range s.Receive() {
			t.Fail()
		}

		wg.Done()
	}()

	s.Disconnect()

	wg.Wait()
}

func TestBoltTransportDispatch(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", transport.logger)
	s.SetTopics([]string{"https://example.com/foo", "https://example.com/private"}, []string{"https://example.com/private"})

	require.NoError(t, transport.AddSubscriber(s))

	notSubscribed := &Update{Topics: []string{"not-subscribed"}}
	require.NoError(t, transport.Dispatch(notSubscribed))

	subscribedNotAuthorized := &Update{Topics: []string{"https://example.com/foo"}, Private: true}
	require.NoError(t, transport.Dispatch(subscribedNotAuthorized))

	public := &Update{Topics: s.SubscribedTopics}
	require.NoError(t, transport.Dispatch(public))

	assert.Equal(t, public, <-s.Receive())

	private := &Update{Topics: s.AllowedPrivateTopics, Private: true}
	require.NoError(t, transport.Dispatch(private))

	assert.Equal(t, private, <-s.Receive())
}

func TestBoltTransportClosed(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", transport.logger)
	s.SetTopics([]string{"https://example.com/foo"}, nil)
	require.NoError(t, transport.AddSubscriber(s))

	require.NoError(t, transport.Close())
	require.Error(t, transport.AddSubscriber(s))

	assert.Equal(t, transport.Dispatch(&Update{Topics: s.SubscribedTopics}), ErrClosedTransport)

	_, ok := <-s.out
	assert.False(t, ok)
}

func TestBoltCleanDisconnectedSubscribers(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")

	s1 := NewSubscriber("", transport.logger)
	s1.SetTopics([]string{"foo"}, []string{})
	require.NoError(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", transport.logger)
	s2.SetTopics([]string{"foo"}, []string{})
	require.NoError(t, transport.AddSubscriber(s2))

	assert.Equal(t, 2, transport.subscribers.Len())

	s1.Disconnect()
	transport.RemoveSubscriber(s1)
	assert.Equal(t, 1, transport.subscribers.Len())

	s2.Disconnect()
	transport.RemoveSubscriber(s2)
	assert.Zero(t, transport.subscribers.Len())
}

func TestBoltGetSubscribers(t *testing.T) {
	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")

	s1 := NewSubscriber("", transport.logger)
	require.NoError(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", transport.logger)
	require.NoError(t, transport.AddSubscriber(s2))

	lastEventID, subscribers, err := transport.GetSubscribers()
	require.NoError(t, err)

	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, s1)
	assert.Contains(t, subscribers, s2)
}

func TestBoltLastEventID(t *testing.T) {
	db, err := bolt.Open("test.db", 0o600, nil)
	defer os.Remove("test.db")
	require.NoError(t, err)

	db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(defaultBoltBucketName))
		require.NoError(t, err)

		seq, err := bucket.NextSequence()
		require.NoError(t, err)

		prefix := make([]byte, 8)
		binary.BigEndian.PutUint64(prefix, seq)

		// The sequence value is prepended to the update id to create an ordered list
		key := bytes.Join([][]byte{prefix, []byte("foo")}, []byte{})

		// The DB is append only
		bucket.FillPercent = 1

		return bucket.Put(key, []byte("invalid"))
	})
	require.NoError(t, db.Close())

	transport := createBoltTransport("bolt://test.db")
	require.NotNil(t, transport)
	defer transport.Close()

	lastEventID, _, _ := transport.GetSubscribers()
	assert.Equal(t, "foo", lastEventID)
}
