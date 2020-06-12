package hub

import (
	"os"
	"sync"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalTransportDoNotDispatchUntilListen(t *testing.T) {
	transport := NewLocalTransport()
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	u := &Update{Topics: []string{"http://example.com/books/1"}}
	err := transport.Dispatch(u)
	require.Nil(t, err)

	s := NewSubscriber("", NewTopicSelectorStore())
	s.Topics = u.Topics
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	var (
		wg         sync.WaitGroup
		readUpdate *Update
		ok         bool
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case readUpdate = <-s.Receive():
		case <-s.disconnected:
			ok = true
		}
	}()

	s.Disconnect()

	wg.Wait()
	assert.Nil(t, readUpdate)
	assert.True(t, ok)
}

func TestLocalTransportDispatch(t *testing.T) {
	transport := NewLocalTransport()
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", NewTopicSelectorStore())
	s.Topics = []string{"http://example.com/foo"}
	go s.start()
	assert.Nil(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.Topics}
	require.Nil(t, transport.Dispatch(u))
	assert.Equal(t, u, <-s.Receive())
}

func TestLocalTransportClosed(t *testing.T) {
	transport := NewLocalTransport()
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	tss := NewTopicSelectorStore()

	s := NewSubscriber("", tss)
	require.Nil(t, transport.AddSubscriber(s))

	assert.Nil(t, transport.Close())
	assert.Equal(t, transport.AddSubscriber(NewSubscriber("", tss)), ErrClosedTransport)
	assert.Equal(t, transport.Dispatch(&Update{}), ErrClosedTransport)

	_, ok := <-s.disconnected
	assert.False(t, ok)
}

func TestLiveCleanDisconnectedSubscribers(t *testing.T) {
	transport := NewLocalTransport()
	defer transport.Close()

	tss := NewTopicSelectorStore()

	s1 := NewSubscriber("", tss)
	go s1.start()
	require.Nil(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", tss)
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

func TestLiveReading(t *testing.T) {
	transport := NewLocalTransport()
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", NewTopicSelectorStore())
	s.Topics = []string{"https://example.com"}
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.Topics}
	assert.Nil(t, transport.Dispatch(u))

	receivedUpdate := <-s.Receive()
	assert.Equal(t, u, receivedUpdate)
}

func TestNewTransport(t *testing.T) {
	transport, err := NewTransport(viper.New())
	assert.Nil(t, err)
	require.NotNil(t, transport)
	transport.Close()
	assert.IsType(t, &LocalTransport{}, transport)

	v := viper.New()
	v.Set("transport_url", "bolt://test.db")
	transport, _ = NewTransport(v)
	assert.Nil(t, err)
	require.NotNil(t, transport)
	transport.Close()
	os.Remove("test.db")
	assert.IsType(t, &BoltTransport{}, transport)

	v = viper.New()
	v.Set("transport_url", "nothing:")
	transport, err = NewTransport(v)
	assert.Nil(t, transport)
	assert.NotNil(t, err)
	assert.EqualError(t, err, `"nothing:": no such transport available: invalid transport DSN`)

	v = viper.New()
	v.Set("transport_url", "http://[::1]%23")
	_, err = NewTransport(v)
	assert.EqualError(t, err, `transport_url: parse "http://[::1]%23": invalid port "%23" after host`)
}

func TestLocalTransportGetSubscribers(t *testing.T) {
	transport := NewLocalTransport()
	defer transport.Close()
	require.NotNil(t, transport)

	tss := NewTopicSelectorStore()

	s1 := NewSubscriber("", tss)
	go s1.start()
	require.Nil(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", tss)
	go s2.start()
	require.Nil(t, transport.AddSubscriber(s2))

	lastEventID, subscribers := transport.GetSubscribers()
	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, s1)
	assert.Contains(t, subscribers, s2)
}
