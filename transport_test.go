package mercure

import (
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLocalTransportDoNotDispatchUntilListen(t *testing.T) {
	transport, _ := NewLocalTransport(&url.URL{Scheme: "local"}, zap.NewNop(), nil)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	u := &Update{Topics: []string{"http://example.com/books/1"}}
	err := transport.Dispatch(u)
	require.Nil(t, err)

	s := NewSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.Topics = u.Topics
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range s.Receive() {
			t.Fail()
		}
	}()

	s.Disconnect()
	wg.Wait()
}

func TestLocalTransportDispatch(t *testing.T) {
	transport, _ := NewLocalTransport(&url.URL{Scheme: "local"}, zap.NewNop(), nil)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.Topics = []string{"http://example.com/foo"}
	go s.start()
	assert.Nil(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.Topics}
	require.Nil(t, transport.Dispatch(u))
	assert.Equal(t, u, <-s.Receive())
}

func TestLocalTransportClosed(t *testing.T) {
	transport, _ := NewLocalTransport(&url.URL{Scheme: "local"}, zap.NewNop(), nil)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	tss := &TopicSelectorStore{}
	s := NewSubscriber("", zap.NewNop(), tss)
	require.Nil(t, transport.AddSubscriber(s))

	assert.Nil(t, transport.Close())
	assert.Equal(t, transport.AddSubscriber(NewSubscriber("", zap.NewNop(), tss)), ErrClosedTransport)
	assert.Equal(t, transport.Dispatch(&Update{}), ErrClosedTransport)

	_, ok := <-s.disconnected
	assert.False(t, ok)
}

func TestLiveCleanDisconnectedSubscribers(t *testing.T) {
	tr, _ := NewLocalTransport(&url.URL{Scheme: "local"}, zap.NewNop(), nil)
	transport := tr.(*LocalTransport)
	defer transport.Close()

	tss := &TopicSelectorStore{}
	s1 := NewSubscriber("", zap.NewNop(), tss)
	go s1.start()
	require.Nil(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", zap.NewNop(), tss)
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
	transport, _ := NewLocalTransport(&url.URL{Scheme: "local"}, zap.NewNop(), nil)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.Topics = []string{"https://example.com"}
	go s.start()
	require.Nil(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.Topics}
	assert.Nil(t, transport.Dispatch(u))

	receivedUpdate := <-s.Receive()
	assert.Equal(t, u, receivedUpdate)
}

func TestLocalTransportGetSubscribers(t *testing.T) {
	transport, _ := NewLocalTransport(&url.URL{Scheme: "local"}, zap.NewNop(), nil)
	defer transport.Close()
	require.NotNil(t, transport)

	tss := &TopicSelectorStore{}
	s1 := NewSubscriber("", zap.NewNop(), tss)
	go s1.start()
	require.Nil(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", zap.NewNop(), tss)
	go s2.start()
	require.Nil(t, transport.AddSubscriber(s2))

	lastEventID, subscribers, err := transport.(TransportSubscribers).GetSubscribers()
	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, s1)
	assert.Contains(t, subscribers, s2)
	assert.Nil(t, err)
}
