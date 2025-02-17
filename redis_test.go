package mercure

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	redisHost                        = "localhost:6379"
	redisSubscriberSize              = 100000
	redisSubscriberBroadcastParallel = 16
)

func initialize() *RedisTransport {
	transport, _ := NewRedisTransport(zap.NewNop(), redisHost, "", "", redisSubscriberSize, redisSubscriberBroadcastParallel)

	return transport
}

func TestRedisWaitListen(t *testing.T) {
	transport := initialize()
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)
	s := NewLocalSubscriber("", transport.logger, &TopicSelectorStore{})
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

func TestRedisDispatch(t *testing.T) {
	transport := initialize()
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)

	subscriber := NewLocalSubscriber("", transport.logger, &TopicSelectorStore{})
	subscriber.SetTopics([]string{"https://topics.local/topic", "https://topics.local/private"}, []string{"https://topics.local/private"})
	require.NoError(t, transport.AddSubscriber(subscriber))
	notSubscribed := &Update{Topics: []string{"not-subscribed"}}
	require.NoError(t, transport.Dispatch(notSubscribed))
	subscribedSkipped := &Update{Topics: []string{"https://topics.local/topic"}, Private: true}
	require.NoError(t, transport.Dispatch(subscribedSkipped))
	public := &Update{Topics: subscriber.SubscribedTopics}
	require.NoError(t, transport.Dispatch(public))
	assert.Equal(t, public, <-subscriber.Receive())
	private := &Update{Topics: subscriber.AllowedPrivateTopics, Private: true}
	require.NoError(t, transport.Dispatch(private))
	assert.Equal(t, private, <-subscriber.Receive())
}

func TestRedisClose(t *testing.T) {
	transport := initialize()
	require.NotNil(t, transport)
	defer transport.Close()
	assert.Implements(t, (*Transport)(nil), transport)
	subscriber := NewLocalSubscriber("", transport.logger, &TopicSelectorStore{})
	subscriber.SetTopics([]string{"https://topics.local/topic"}, nil)
	require.NoError(t, transport.AddSubscriber(subscriber))
	require.NoError(t, transport.Close())
	require.Error(t, transport.AddSubscriber(subscriber))
	assert.Equal(t, transport.Dispatch(&Update{Topics: subscriber.SubscribedTopics}), ErrClosedTransport)
	_, ok := <-subscriber.out
	assert.False(t, ok)
}

func TestRedisConcurrent(t *testing.T) {
	transport1 := initialize()
	transport2 := initialize()
	transport3 := initialize()

	defer transport1.Close()
	defer transport2.Close()
	defer transport3.Close()

	topics := []string{"https://topics.local/topic1", "https://topics.local/topic2", "https://topics.local/topic3"}

	const transport1EventsCount = 100
	const transport2EventsCount = 10
	const transport3EventsCount = 1

	const transport1SubscribersCount = 5
	const transport2SubscribersCount = 25
	const transport3SubscribersCount = 50

	wg := sync.WaitGroup{}
	wg.Add((transport1EventsCount + transport2EventsCount + transport3EventsCount) * (transport1SubscribersCount + transport2SubscribersCount + transport3SubscribersCount))

	counter1 := atomic.Int64{}
	counter2 := atomic.Int64{}
	counter3 := atomic.Int64{}

	transport1Subscribers := []*LocalSubscriber{}
	transport2Subscribers := []*LocalSubscriber{}
	transport3Subscribers := []*LocalSubscriber{}

	defer func() {
		if recover() != nil {
			t.Logf("Counter 1 = %d\n", counter1.Load())
			t.Logf("Counter 2 = %d\n", counter2.Load())
			t.Logf("Counter 3 = %d\n", counter3.Load())
		}
	}()

	for range transport1SubscribersCount {
		subscriber := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
		subscriber.SetTopics(topics, nil)
		transport1.AddSubscriber(subscriber)
		transport1Subscribers = append(transport1Subscribers, subscriber)
	}

	for range transport2SubscribersCount {
		subscriber := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
		subscriber.SetTopics(topics, nil)
		transport2.AddSubscriber(subscriber)
		transport2Subscribers = append(transport2Subscribers, subscriber)
	}

	for range transport3SubscribersCount {
		subscriber := NewLocalSubscriber("", zap.NewNop(), &TopicSelectorStore{})
		subscriber.SetTopics(topics, nil)
		transport3.AddSubscriber(subscriber)
		transport3Subscribers = append(transport3Subscribers, subscriber)
	}

	for range transport1EventsCount {
		update := Update{Topics: topics, Event: Event{Data: "test1"}}
		go transport1.Dispatch(&update)
	}

	for range transport2EventsCount {
		update := Update{Topics: topics, Event: Event{Data: "test2"}}
		go transport2.Dispatch(&update)
	}

	for range transport3EventsCount {
		update := Update{Topics: topics, Event: Event{Data: "test3"}}
		go transport3.Dispatch(&update)
	}

	for _, subscriber := range transport1Subscribers {
		go func() {
			for range subscriber.Receive() {
				counter1.Add(1)
				wg.Done()
			}
		}()
	}

	for _, subscriber := range transport2Subscribers {
		go func() {
			for range subscriber.Receive() {
				counter2.Add(1)
				wg.Done()
			}
		}()
	}

	for _, subscriber := range transport3Subscribers {
		go func() {
			for range subscriber.Receive() {
				counter3.Add(1)
				wg.Done()
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(transport1EventsCount+transport2EventsCount+transport3EventsCount)*transport1SubscribersCount, counter1.Load())
	assert.Equal(t, int64(transport1EventsCount+transport2EventsCount+transport3EventsCount)*transport2SubscribersCount, counter2.Load())
	assert.Equal(t, int64(transport1EventsCount+transport2EventsCount+transport3EventsCount)*transport3SubscribersCount, counter3.Load())
}
