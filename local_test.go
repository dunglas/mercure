package mercure

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLocalTransportDoNotDispatchUntilListen(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport()
	defer transport.Close()

	assert.Implements(t, (*Transport)(nil), transport)

	u := &Update{Topics: []string{"http://example.com/books/1"}}
	err := transport.Dispatch(u)
	require.NoError(t, err)

	s := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.SetTopics(u.Topics, nil)
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

func TestLocalTransportDispatch(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport()
	defer transport.Close()

	assert.Implements(t, (*Transport)(nil), transport)

	s := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.SetTopics([]string{"http://example.com/foo"}, nil)
	require.NoError(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.SubscribedTopics}
	require.NoError(t, transport.Dispatch(u))
	assert.Equal(t, u, <-s.Receive())
}

func TestLocalTransportClosed(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport()
	defer transport.Close()

	assert.Implements(t, (*Transport)(nil), transport)

	tss := &TopicSelectorStore{}
	logger := zap.NewNop()

	s := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s))
	require.NoError(t, transport.Close())
	assert.Equal(t, transport.AddSubscriber(NewLocalSubscriber("", logger, tss)), ErrClosedTransport)
	assert.Equal(t, transport.Dispatch(&Update{}), ErrClosedTransport)

	_, ok := <-s.Receive()
	assert.False(t, ok)
}

func TestLiveCleanDisconnectedSubscribers(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport()
	defer transport.Close()

	tss := &TopicSelectorStore{}
	logger := zap.NewNop()

	s1 := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s1))

	s2 := NewLocalSubscriber("", logger, tss)
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
	t.Parallel()

	transport := NewLocalTransport()
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	s := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
	s.SetTopics([]string{"https://example.com"}, nil)
	require.NoError(t, transport.AddSubscriber(s))

	u := &Update{Topics: s.SubscribedTopics}
	require.NoError(t, transport.Dispatch(u))

	receivedUpdate := <-s.Receive()
	assert.Equal(t, u, receivedUpdate)
}

func TestLocalTransportGetSubscribers(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport()
	defer transport.Close()

	require.NotNil(t, transport)

	logger := zap.NewNop()
	tss := &TopicSelectorStore{}

	s1 := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s1))

	s2 := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(s2))

	lastEventID, subscribers, err := transport.GetSubscribers()
	require.NoError(t, err)
	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, &s1.Subscriber)
	assert.Contains(t, subscribers, &s2.Subscriber)
}
