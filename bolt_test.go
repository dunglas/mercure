package mercure

import (
	"bytes"
	"context"
	"encoding/binary"
	"log/slog"
	"os"
	"strconv"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func createBoltTransport(t *testing.T, size uint64, cleanupFrequency float64) *BoltTransport {
	t.Helper()

	if cleanupFrequency == 0 {
		cleanupFrequency = BoltDefaultCleanupFrequency
	}

	path := "test-" + t.Name() + ".db"
	transport, err := NewBoltTransport(NewSubscriberList(0), slog.Default(), path, defaultBoltBucketName, size, cleanupFrequency)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, os.Remove(path))
		require.NoError(t, transport.Close(t.Context()))
	})

	return transport
}

func TestBoltTransportHistory(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: []string{topics[0]},
		}))
	}

	s := NewLocalSubscriber("8", transport.logger, &TopicMatcherStore{})
	s.setMatchers(stringsToExactMatchers(topics), stringsToExactMatchers(nil))

	require.NoError(t, transport.AddSubscriber(t.Context(), s))

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
	t.Parallel()

	var buf bytes.Buffer

	transport := createBoltTransport(t, 0, 0)
	transport.logger = slog.New(mercureHandler{slog.NewJSONHandler(&buf, nil)})

	topics := []string{"https://example.com/foo"}
	s := NewLocalSubscriber("711131", transport.logger, &TopicMatcherStore{})
	s.setMatchers(stringsToExactMatchers(topics), stringsToExactMatchers(nil))
	ctx := context.WithValue(t.Context(), SubscriberContextKey, &s.Subscriber)

	require.NoError(t, transport.Dispatch(ctx, &Update{Topics: []string{topics[0]}})) // make sure the db is not empty
	require.NoError(t, transport.AddSubscriber(ctx, s))
	assert.Contains(t, buf.String(), `"last_event_id":"711131"`)
}

func TestBoltTopicMatcherHistory(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)
	ctx := t.Context()

	require.NoError(t, transport.Dispatch(ctx, &Update{Topics: []string{"https://example.com/subscribed"}, Event: Event{ID: "1"}}))
	require.NoError(t, transport.Dispatch(ctx, &Update{Topics: []string{"https://example.com/not-subscribed"}, Event: Event{ID: "2"}}))
	require.NoError(t, transport.Dispatch(ctx, &Update{Topics: []string{"https://example.com/subscribed-public-only"}, Private: true, Event: Event{ID: "3"}}))
	require.NoError(t, transport.Dispatch(ctx, &Update{Topics: []string{"https://example.com/subscribed-public-only"}, Event: Event{ID: "4"}}))

	s := NewLocalSubscriber(EarliestLastEventID, transport.logger, &TopicMatcherStore{})
	s.setMatchers(stringsToExactMatchers([]string{"https://example.com/subscribed", "https://example.com/subscribed-public-only"}), stringsToExactMatchers([]string{"https://example.com/subscribed"}))

	require.NoError(t, transport.AddSubscriber(ctx, s))

	assert.Equal(t, "1", (<-s.Receive()).ID)
	assert.Equal(t, "4", (<-s.Receive()).ID)
}

func TestBoltTransportRetrieveAllHistory(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)
	ctx := t.Context()

	topics := []string{"https://example.com/foo"}
	for i := 1; i <= 10; i++ {
		require.NoError(t, transport.Dispatch(ctx, &Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: []string{topics[0]},
		}))
	}

	s := NewLocalSubscriber(EarliestLastEventID, transport.logger, &TopicMatcherStore{})
	s.setMatchers(stringsToExactMatchers(topics), stringsToExactMatchers(nil))
	require.NoError(t, transport.AddSubscriber(ctx, s))

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
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		ctx := t.Context()

		topics := []string{"https://example.com/foo"}
		for i := 1; i <= 10; i++ {
			require.NoError(t, transport.Dispatch(ctx, &Update{
				Topics: []string{topics[0]},
				Event:  Event{ID: strconv.Itoa(i)},
			}))
		}

		s := NewLocalSubscriber("8", transport.logger, &TopicMatcherStore{})
		s.setMatchers(stringsToExactMatchers(topics), stringsToExactMatchers(nil))
		require.NoError(t, transport.AddSubscriber(ctx, s))

		go func() {
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

		require.NoError(t, transport.Dispatch(ctx, &Update{
			Event:  Event{ID: "11"},
			Topics: []string{topics[0]},
		}))

		synctest.Wait()
	})
}

func TestBoltTransportPurgeHistory(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 5, 1)

	for i := range 12 {
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: []string{"https://example.com/foo"},
		}))
	}

	require.NoError(t, transport.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("updates"))

		assert.Equal(t, 5, b.Stats().KeyN)

		return nil
	}))
}

func TestBoltTransportDoNotDispatchUntilListen(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		transport := createBoltTransport(t, 0, 0)
		assert.Implements(t, (*Transport)(nil), transport)

		s := NewLocalSubscriber("", transport.logger, &TopicMatcherStore{})
		require.NoError(t, transport.AddSubscriber(t.Context(), s))

		go func() {
			for range s.Receive() {
				t.Fail()
			}
		}()

		s.Disconnect()

		synctest.Wait()
	})
}

func TestBoltTransportDispatch(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)
	assert.Implements(t, (*Transport)(nil), transport)

	ctx := t.Context()

	s := NewLocalSubscriber("", transport.logger, &TopicMatcherStore{})
	s.setMatchers(stringsToExactMatchers([]string{"https://example.com/foo", "https://example.com/private"}), stringsToExactMatchers([]string{"https://example.com/private"}))

	require.NoError(t, transport.AddSubscriber(ctx, s))

	notSubscribed := &Update{Topics: []string{"not-subscribed"}}
	require.NoError(t, transport.Dispatch(ctx, notSubscribed))

	subscribedNotAuthorized := &Update{Topics: []string{"https://example.com/foo"}, Private: true}
	require.NoError(t, transport.Dispatch(ctx, subscribedNotAuthorized))

	public := &Update{Topics: []string{s.SubscribedMatchers[0].Pattern}}
	require.NoError(t, transport.Dispatch(ctx, public))

	assert.Equal(t, public, <-s.Receive())

	private := &Update{Topics: []string{s.AllowedPrivateMatchers[0].Pattern}, Private: true}
	require.NoError(t, transport.Dispatch(ctx, private))

	assert.Equal(t, private, <-s.Receive())
}

func TestBoltTransportClosed(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)
	assert.Implements(t, (*Transport)(nil), transport)

	ctx := t.Context()

	s := NewLocalSubscriber("", transport.logger, &TopicMatcherStore{})
	s.setMatchers(stringsToExactMatchers([]string{"https://example.com/foo"}), stringsToExactMatchers(nil))
	require.NoError(t, transport.AddSubscriber(ctx, s))

	require.NoError(t, transport.Close(ctx))
	require.Error(t, transport.AddSubscriber(ctx, s))

	assert.Equal(t, transport.Dispatch(ctx, &Update{Topics: []string{s.SubscribedMatchers[0].Pattern}}), ErrClosedTransport)

	_, ok := <-s.Receive()
	assert.False(t, ok)
}

func TestBoltCleanDisconnectedSubscribers(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)
	ctx := t.Context()

	s1 := NewLocalSubscriber("", transport.logger, &TopicMatcherStore{})
	s1.setMatchers(stringsToExactMatchers([]string{"foo"}), stringsToExactMatchers([]string{}))
	require.NoError(t, transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", transport.logger, &TopicMatcherStore{})
	s2.setMatchers(stringsToExactMatchers([]string{"foo"}), stringsToExactMatchers([]string{}))
	require.NoError(t, transport.AddSubscriber(ctx, s2))

	assert.Equal(t, 2, transport.subscribers.Len())

	s1.Disconnect()
	require.NoError(t, transport.RemoveSubscriber(ctx, s1))
	assert.Equal(t, 1, transport.subscribers.Len())

	s2.Disconnect()
	require.NoError(t, transport.RemoveSubscriber(ctx, s2))
	assert.Zero(t, transport.subscribers.Len())
}

func TestBoltGetSubscribers(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)
	ctx := t.Context()

	s1 := NewLocalSubscriber("", transport.logger, &TopicMatcherStore{})
	require.NoError(t, transport.AddSubscriber(ctx, s1))

	s2 := NewLocalSubscriber("", transport.logger, &TopicMatcherStore{})
	require.NoError(t, transport.AddSubscriber(ctx, s2))

	lastEventID, subscribers, err := transport.GetSubscribers(ctx)
	require.NoError(t, err)

	assert.Equal(t, EarliestLastEventID, lastEventID)
	assert.Len(t, subscribers, 2)
	assert.Contains(t, subscribers, &s1.Subscriber)
	assert.Contains(t, subscribers, &s2.Subscriber)
}

func TestBoltLastEventID(t *testing.T) {
	t.Parallel()

	path := "test-" + t.Name() + ".db"
	db, err := bolt.Open(path, 0o600, nil)

	t.Cleanup(func() {
		_ = os.Remove(path)
	})
	require.NoError(t, err)

	require.NoError(t, db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(defaultBoltBucketName))
		require.NoError(t, err)

		seq, err := bucket.NextSequence()
		require.NoError(t, err)

		prefix := make([]byte, 8)
		binary.BigEndian.PutUint64(prefix, seq)

		// The sequence value is prepended to the update id to create an ordered list
		key := bytes.Join([][]byte{prefix, []byte("foo")}, []byte{})

		// The DB is append-only
		bucket.FillPercent = 1

		return bucket.Put(key, []byte("invalid"))
	}))
	require.NoError(t, db.Close())

	transport := createBoltTransport(t, 0, 0)

	lastEventID, _, _ := transport.GetSubscribers(t.Context())
	assert.Equal(t, "foo", lastEventID)
}

// cleanup_frequency is documented as the probability of running a cleanup pass
// on each publish, so a higher value must clean more often, not less.
func TestBoltTransportCleanupFrequencyIsAProbability(t *testing.T) {
	t.Parallel()

	const samples = 20000

	for _, tc := range []struct {
		frequency float64
		want      float64
	}{
		{frequency: 0, want: 0},
		{frequency: 0.1, want: 0.1},
		{frequency: 0.3, want: 0.3},
		{frequency: 0.9, want: 0.9},
		{frequency: 1, want: 1},
	} {
		t.Run(strconv.FormatFloat(tc.frequency, 'f', -1, 64), func(t *testing.T) {
			t.Parallel()

			transport := &BoltTransport{size: 5, cleanupFrequency: tc.frequency}

			var runs int

			for range samples {
				// lastID is past the size limit, so only the probability decides.
				if transport.shouldCleanup(100) {
					runs++
				}
			}

			assert.InDelta(t, tc.want, float64(runs)/samples, 0.02)
		})
	}
}

func TestBoltTransportCleanupSkippedWithinSizeLimit(t *testing.T) {
	t.Parallel()

	// Always-on frequency, but the history has not reached the limit yet.
	transport := &BoltTransport{size: 5, cleanupFrequency: 1}
	assert.False(t, transport.shouldCleanup(5))
	assert.True(t, transport.shouldCleanup(6))

	// An unlimited history is never cleaned.
	unlimited := &BoltTransport{size: 0, cleanupFrequency: 1}
	assert.False(t, unlimited.shouldCleanup(1_000_000))
}

// A requested Last-Event-ID that is not in the history means nothing was
// replayed, so there is no "event preceding the first one sent". The response
// cursor must be the reserved "earliest" value rather than the newest id seen
// while searching, which the subscriber never received.
func TestBoltTransportUnknownLastEventIDReportsEarliest(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)

	for i := range 5 {
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: []string{"https://example.com/foo"},
		}))
	}

	s := NewLocalSubscriber("does-not-exist", transport.logger, &TopicMatcherStore{})
	s.SetMatchers([]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/foo"}}, nil)
	require.NoError(t, transport.AddSubscriber(t.Context(), s))

	assert.Equal(t, EarliestLastEventID, <-s.responseLastEventID)

	s.Disconnect()
}

// A known id is echoed back: the subscriber already has it, and it really is
// the event preceding the first one replayed.
func TestBoltTransportKnownLastEventIDIsEchoed(t *testing.T) {
	t.Parallel()

	transport := createBoltTransport(t, 0, 0)

	for i := range 5 {
		require.NoError(t, transport.Dispatch(t.Context(), &Update{
			Event:  Event{ID: strconv.Itoa(i)},
			Topics: []string{"https://example.com/foo"},
		}))
	}

	s := NewLocalSubscriber("2", transport.logger, &TopicMatcherStore{})
	s.SetMatchers([]TopicMatcher{{Type: MatcherTypeExact, Pattern: "https://example.com/foo"}}, nil)
	require.NoError(t, transport.AddSubscriber(t.Context(), s))

	assert.Equal(t, "2", <-s.responseLastEventID)

	s.Disconnect()
}
