package mercure

import (
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalTransportDoNotDispatchUntilListen(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport(NewSubscriberList(0))
	ctx := t.Context()

	t.Cleanup(func() {
		assert.NoError(t, transport.Close(ctx))
	})

	assert.Implements(t, (*Transport)(nil), transport)

	u := &Update{Topics: []string{"https://example.com/books/1"}}
	err := transport.Dispatch(ctx, u)
	require.NoError(t, err)

	s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
	s.SetTopics(u.Topics, nil)
	require.NoError(t, transport.AddSubscriber(ctx, s))

	synctest.Test(t, func(t *testing.T) {
		go func() {
			for range s.Receive() {
				t.Fail()
			}
		}()

		s.Disconnect()
		synctest.Wait()
	})
}

func TestLocalTransportDispatch(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport(NewSubscriberList(0))
	ctx := t.Context()

	t.Cleanup(func() {
		assert.NoError(t, transport.Close(ctx))
	})

	assert.Implements(t, (*Transport)(nil), transport)

	s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
	s.SetTopics([]string{"https://example.com/foo"}, nil)
	require.NoError(t, transport.AddSubscriber(ctx, s))

	u := &Update{Topics: s.SubscribedTopics}
	require.NoError(t, transport.Dispatch(ctx, u))
	assert.Equal(t, u, <-s.Receive())
}

func TestLocalTransportClosed(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport(NewSubscriberList(0))
	ctx := t.Context()

	t.Cleanup(func() {
		assert.NoError(t, transport.Close(ctx))
	})

	assert.Implements(t, (*Transport)(nil), transport)

	tss := &TopicSelectorStore{}
	logger := slog.Default()

	s := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(ctx, s))
	require.NoError(t, transport.Close(ctx))
	assert.Equal(t, transport.AddSubscriber(ctx, NewLocalSubscriber("", logger, tss)), ErrClosedTransport)
	assert.Equal(t, transport.Dispatch(ctx, &Update{}), ErrClosedTransport)

	_, ok := <-s.Receive()
	assert.False(t, ok)
}

func TestLiveCleanDisconnectedSubscribers(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport(NewSubscriberList(0))
	ctx := t.Context()

	t.Cleanup(func() {
		assert.NoError(t, transport.Close(ctx))
	})

	tss := &TopicSelectorStore{}
	logger := slog.Default()

	s1 := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(ctx, s2))

	assert.Equal(t, 2, transport.subscribers.Len())

	s1.Disconnect()
	require.NoError(t, transport.RemoveSubscriber(ctx, s1))
	assert.Equal(t, 1, transport.subscribers.Len())

	s2.Disconnect()
	require.NoError(t, transport.RemoveSubscriber(ctx, s2))
	assert.Equal(t, 0, transport.subscribers.Len())
}

func TestLiveReading(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport(NewSubscriberList(0))
	ctx := t.Context()

	t.Cleanup(func() {
		assert.NoError(t, transport.Close(ctx))
	})

	assert.Implements(t, (*Transport)(nil), transport)

	s := NewLocalSubscriber("", slog.Default(), &TopicSelectorStore{})
	s.SetTopics([]string{"https://example.com"}, nil)
	require.NoError(t, transport.AddSubscriber(ctx, s))

	u := &Update{Topics: s.SubscribedTopics}
	require.NoError(t, transport.Dispatch(ctx, u))

	receivedUpdate := <-s.Receive()
	assert.Equal(t, u, receivedUpdate)
}

func TestLocalTransportGetSubscribers(t *testing.T) {
	t.Parallel()

	transport := NewLocalTransport(NewSubscriberList(0))
	ctx := t.Context()

	t.Cleanup(func() {
		assert.NoError(t, transport.Close(ctx))
	})

	require.NotNil(t, transport)

	tss := &TopicSelectorStore{}
	logger := slog.Default()

	s1 := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", logger, tss)
	require.NoError(t, transport.AddSubscriber(ctx, s2))

	lastEventID, subscribers, err := transport.GetSubscribers(ctx)
	require.NoError(t, err)
	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, &s1.Subscriber)
	assert.Contains(t, subscribers, &s2.Subscriber)
}
