package hub

import (
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestBoltTransportHistory(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		transport.Dispatch(&Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: topics,
		})
	}

	s := newSubscriber("8", newTopicSelectorStore())
	s.Topics = topics
	go s.start()

	err := transport.AddSubscriber(s)
	assert.Nil(t, err)

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
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		transport.Dispatch(&Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: topics,
		})
	}

	s := newSubscriber("-1", newTopicSelectorStore())
	s.Topics = topics
	go s.start()

	err := transport.AddSubscriber(s)
	assert.Nil(t, err)

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
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		transport.Dispatch(&Update{
			Topics: topics,
			Event:  Event{ID: strconv.Itoa(i)},
		})
	}

	s := newSubscriber("8", newTopicSelectorStore())
	s.Topics = topics
	go s.start()

	err := transport.AddSubscriber(s)
	assert.Nil(t, err)

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
	u, _ := url.Parse("bolt://test.db?size=5&cleanup_frequency=1")
	transport, _ := NewBoltTransport(u)
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
	transport, err := NewBoltTransport(u)
	assert.Nil(t, err)
	require.NotNil(t, transport)
	transport.Close()

	u, _ = url.Parse("bolt://")
	_, err = NewBoltTransport(u)
	assert.EqualError(t, err, `"bolt:": missing path: invalid transport DSN`)

	u, _ = url.Parse("bolt:///test.db")
	_, err = NewBoltTransport(u)

	// The exact error message depends of the OS
	assert.Contains(t, err.Error(), "open /test.db:")

	u, _ = url.Parse("bolt://test.db?cleanup_frequency=invalid")
	_, err = NewBoltTransport(u)
	assert.EqualError(t, err, `"bolt://test.db?cleanup_frequency=invalid": invalid "cleanup_frequency" parameter "invalid": invalid transport DSN`)

	u, _ = url.Parse("bolt://test.db?size=invalid")
	_, err = NewBoltTransport(u)
	assert.EqualError(t, err, `"bolt://test.db?size=invalid": invalid "size" parameter "invalid": strconv.ParseUint: parsing "invalid": invalid syntax: invalid transport DSN`)
}

func TestBoltTransportDoNotDispatchedUntilListen(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := newSubscriber("", newTopicSelectorStore())
	go s.start()

	err := transport.AddSubscriber(s)
	assert.Nil(t, err)

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
	ur, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(ur)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := newSubscriber("", newTopicSelectorStore())
	s.Topics = []string{"https://example.com/foo"}
	go s.start()

	err := transport.AddSubscriber(s)
	assert.Nil(t, err)

	u := &Update{Topics: s.Topics}

	err = transport.Dispatch(u)
	assert.Nil(t, err)

	readUpdate := <-s.Receive()
	assert.Equal(t, u, readUpdate)
}

func TestBoltTransportClosed(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")
	assert.Implements(t, (*Transport)(nil), transport)

	s := newSubscriber("", newTopicSelectorStore())
	s.Topics = []string{"https://example.com/foo"}
	go s.start()

	err := transport.AddSubscriber(s)
	require.Nil(t, err)

	err = transport.Close()
	assert.Nil(t, err)

	err = transport.AddSubscriber(s)
	assert.Equal(t, err, ErrClosedTransport)

	err = transport.Dispatch(&Update{Topics: s.Topics})
	assert.Equal(t, err, ErrClosedTransport)

	_, ok := <-s.disconnected
	assert.False(t, ok)
}

func TestBoltCleanDisconnectedSubscribers(t *testing.T) {
	u, _ := url.Parse("bolt://test.db")
	transport, _ := NewBoltTransport(u)
	require.NotNil(t, transport)
	defer transport.Close()
	defer os.Remove("test.db")

	tss := newTopicSelectorStore()

	s1 := newSubscriber("", tss)
	go s1.start()
	err := transport.AddSubscriber(s1)
	require.Nil(t, err)

	s2 := newSubscriber("", tss)
	go s2.start()
	err = transport.AddSubscriber(s2)
	require.Nil(t, err)

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
