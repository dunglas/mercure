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
	logger := zap.NewNop()
	transport, _ := DeprecatedNewLocalTransport(&url.URL{Scheme: "local"}, logger)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	u := &Update{Topics: []string{"http://example.com/books/1"}}
	err := transport.Dispatch(u)
	require.NoError(t, err)

	s := NewSubscriber("", logger, &TopicSelectorStore{})
	s.SetTopics(u.Topics, nil)
	require.NoError(t, transport.AddSubscriber(s))

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
	logger := zap.NewNop()
	transport, _ := DeprecatedNewLocalTransport(&url.URL{Scheme: "local"}, logger)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", logger, &TopicSelectorStore{})
	s.SetTopics([]string{"http://example.com/foo"}, nil)
	require.NoError(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.SubscribedTopics}
	require.NoError(t, transport.Dispatch(u))
	assert.Equal(t, u, <-s.Receive())
}

func TestLocalTransportClosed(t *testing.T) {
	logger := zap.NewNop()
	transport, _ := DeprecatedNewLocalTransport(&url.URL{Scheme: "local"}, logger)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	tss := &TopicSelectorStore{}

	s := NewSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s))
	require.NoError(t, transport.Close())
	assert.Equal(t, transport.AddSubscriber(NewSubscriber("", logger, tss)), ErrClosedTransport)
	assert.Equal(t, transport.Dispatch(&Update{}), ErrClosedTransport)

	_, ok := <-s.out
	assert.False(t, ok)
}

func TestLiveCleanDisconnectedSubscribers(t *testing.T) {
	logger := zap.NewNop()
	tr, _ := DeprecatedNewLocalTransport(&url.URL{Scheme: "local"}, logger)
	transport := tr.(*LocalTransport)
	defer transport.Close()

	tss := &TopicSelectorStore{}

	s1 := NewSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s2))

	assert.Equal(t, 2, transport.subscribers.Len())

	s1.Disconnect()
	transport.RemoveSubscriber(s1)
	assert.Equal(t, 1, transport.subscribers.Len())

	s2.Disconnect()
	transport.RemoveSubscriber(s2)
	assert.Equal(t, 0, transport.subscribers.Len())
}

func TestLiveReading(t *testing.T) {
	logger := zap.NewNop()
	transport, _ := DeprecatedNewLocalTransport(&url.URL{Scheme: "local"}, logger)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewSubscriber("", logger, &TopicSelectorStore{})
	s.SetTopics([]string{"https://example.com"}, nil)
	require.NoError(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.SubscribedTopics}
	require.NoError(t, transport.Dispatch(u))

	receivedUpdate := <-s.Receive()
	assert.Equal(t, u, receivedUpdate)
}

func TestLocalTransportGetSubscribers(t *testing.T) {
	logger := zap.NewNop()
	transport, _ := DeprecatedNewLocalTransport(&url.URL{Scheme: "local"}, logger)
	defer transport.Close()
	require.NotNil(t, transport)

	tss := &TopicSelectorStore{}

	s1 := NewSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s1))

	s2 := NewSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s2))

	lastEventID, subscribers, err := transport.(TransportSubscribers).GetSubscribers()
	require.NoError(t, err)
	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, s1)
	assert.Contains(t, subscribers, s2)
}
