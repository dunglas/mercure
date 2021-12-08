package mercure

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"sync"

	redis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func init() { //nolint:gochecknoinits
	RegisterTransportFactory("redis", NewRedisTransport)
}

const RedisDefaultCleanupFrequency = 0.3

const defaultRedisBucketName = "updates"

// RedisTransport implements the TransportInterface using the Bolt database.
type RedisTransport struct {
	sync.RWMutex
	logger           Logger
	client           *redis.Client
	ctx              context.Context
	bucketName       string
	size             uint64
	cleanupFrequency float64
	subscribers      map[*Subscriber]struct{}
	closed           chan struct{}
	closedOnce       sync.Once
	lastEventID      string
}

// NewRedisTransport create a new RedisTransport.
func NewRedisTransport(u *url.URL, l Logger, tss *TopicSelectorStore) (Transport, error) { //nolint:ireturn
	var err error
	q := u.Query()
	bucketName := defaultRedisBucketName
	if q.Get("bucket_name") != "" {
		bucketName = q.Get("bucket_name")
	}

	size := uint64(0)
	if sizeParameter := q.Get("size"); sizeParameter != "" {
		size, err = strconv.ParseUint(sizeParameter, 10, 64)
		if err != nil {
			return nil, &TransportError{u.Redacted(), fmt.Sprintf(`invalid "size" parameter %q`, sizeParameter), err}
		}
	}

	cleanupFrequency := RedisDefaultCleanupFrequency
	cleanupFrequencyParameter := q.Get("cleanup_frequency")
	if cleanupFrequencyParameter != "" {
		cleanupFrequency, err = strconv.ParseFloat(cleanupFrequencyParameter, 64)
		if err != nil {
			return nil, &TransportError{u.Redacted(), fmt.Sprintf(`invalid "cleanup_frequency" parameter %q`, cleanupFrequencyParameter), err}
		}
	}

	path := u.Path // absolute path (bolt:///path.db)
	if path == "" {
		path = u.Host // relative path (bolt://path.db)
	}
	if path == "" {
		return nil, &TransportError{u.Redacted(), "missing path", err}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     path,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	ctx := context.Background()

	redisTransport := &RedisTransport{
		logger:           l,
		client:           client,
		ctx:              ctx,
		bucketName:       bucketName,
		size:             size,
		cleanupFrequency: cleanupFrequency,
		subscribers:      make(map[*Subscriber]struct{}),
		closed:           make(chan struct{}),
		lastEventID:      getRedisLastEventID(ctx, client, bucketName),
	}

	go subscribeToUpdate(redisTransport)

	return redisTransport, nil
}

func subscribeToUpdate(t *RedisTransport) {
	pubsub := t.client.Subscribe(t.ctx, "update")
	ch := pubsub.Channel()
	for msg := range ch {
		var lastUpdate *Update
		errUnmarshal := json.Unmarshal([]byte(msg.Payload), &lastUpdate)
		if errUnmarshal != nil {
			t.logger.Error("error when unmarshaling message", zap.Any("message", msg), zap.Error(errUnmarshal))
		}
		t.dispatch(lastUpdate)
	}
}

func getRedisLastEventID(ctx context.Context, client *redis.Client, bucketName string) string {
	lastEventID := EarliestLastEventID

	lastValue, err := client.LIndex(ctx, bucketName, 0).Result()
	if err == nil {
		var lastUpdate *Update
		errUnmarshal := json.Unmarshal([]byte(lastValue), &lastUpdate)
		if errUnmarshal != nil {
			return lastEventID
		}
		lastEventID = lastUpdate.ID
	}

	return lastEventID
}

// Dispatch dispatches an update to all subscribers and persists it in Bolt DB.
func (t *RedisTransport) Dispatch(update *Update) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	AssignUUID(update)

	t.Lock()
	defer t.Unlock()

	updateJSON, err := json.Marshal(*update)
	if err != nil {
		return fmt.Errorf("error when marshaling update: %w", err)
	}

	if err := t.persist(update.ID, updateJSON); err != nil {
		return err
	}

	// publish in pubsub for others mercure instances to consume the update and dispatch it to its subscribers
	if err := t.client.Publish(t.ctx, "update", updateJSON).Err(); err != nil {
		return fmt.Errorf("error when publishing update: %w", err)
	}

	return nil
}

// Called when a pubsub message is received.
func (t *RedisTransport) dispatch(update *Update) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	defer t.Unlock()

	for subscriber := range t.subscribers {
		if !subscriber.Dispatch(update, false) {
			delete(t.subscribers, subscriber)
		}
	}

	return nil
}

// persist stores update in the database.
func (t *RedisTransport) persist(updateID string, updateJSON []byte) error {
	t.lastEventID = updateID
	err := t.client.LPush(t.ctx, t.bucketName, updateJSON).Err()
	if err != nil {
		return fmt.Errorf("error while persisting to redis: %w", err)
	}

	return t.cleanup()
}

// AddSubscriber adds a new subscriber to the transport.
func (t *RedisTransport) AddSubscriber(s *Subscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	t.subscribers[s] = struct{}{}
	t.Unlock()

	if s.RequestLastEventID != "" {
		t.dispatchHistory(s)
	}

	s.Ready()

	return nil
}

// RemoveSubscriber removes a new subscriber from the transport.
func (t *RedisTransport) RemoveSubscriber(s *Subscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	delete(t.subscribers, s)
	t.Unlock()

	return nil
}

// GetSubscribers get the list of active subscribers.
func (t *RedisTransport) GetSubscribers() (string, []*Subscriber, error) {
	t.RLock()
	defer t.RUnlock()
	subscribers := make([]*Subscriber, len(t.subscribers))

	i := 0
	for subscriber := range t.subscribers {
		subscribers[i] = subscriber
		i++
	}

	return t.lastEventID, subscribers, nil
}

func (t *RedisTransport) dispatchHistory(s *Subscriber) {
	updates, err := t.client.LRange(t.ctx, t.bucketName, 0, int64(t.size)).Result()
	if err != nil {
		s.HistoryDispatched(EarliestLastEventID)

		return
	}

	responseLastEventID := EarliestLastEventID
	afterFromID := s.RequestLastEventID == EarliestLastEventID
	for _, update := range updates {
		var lastUpdate *Update
		errUnmarshal := json.Unmarshal([]byte(update), &lastUpdate)
		if errUnmarshal != nil {
			s.HistoryDispatched(responseLastEventID)
			t.logger.Error("error when unmarshaling update", zap.String("update", update), zap.Error(errUnmarshal))

			return
		}

		if !afterFromID {
			responseLastEventID = lastUpdate.ID
			if responseLastEventID == s.RequestLastEventID {
				afterFromID = true
			}

			continue
		}

		if !s.Dispatch(lastUpdate, true) {
			s.HistoryDispatched(responseLastEventID)

			return
		}

		return
	}

	s.HistoryDispatched(responseLastEventID)
}

// Close closes the Transport.
func (t *RedisTransport) Close() (err error) {
	t.closedOnce.Do(func() {
		close(t.closed)

		t.Lock()
		for subscriber := range t.subscribers {
			subscriber.Disconnect()
			delete(t.subscribers, subscriber)
		}
		t.Unlock()

		err = t.client.Close()
	})

	if err == nil {
		return nil
	}

	return fmt.Errorf("unable to close Bolt DB: %w", err)
}

// cleanup removes entries in the history above the size limit, triggered probabilistically.
func (t *RedisTransport) cleanup() error {
	sizeUpdates, errLen := t.client.LLen(t.ctx, "updates").Result()
	if errLen != nil {
		return fmt.Errorf("error when getting updates length: %w", errLen)
	}

	if t.size == 0 ||
		t.cleanupFrequency == 0 ||
		t.size >= uint64(sizeUpdates) ||
		(t.cleanupFrequency != 1 && rand.Float64() < t.cleanupFrequency) { //nolint:gosec
		return nil
	}

	errTrim := t.client.LTrim(t.ctx, "updates", 0, int64(t.size)).Err()
	if errTrim != nil {
		return fmt.Errorf("error when trimming update length: %w", errLen)
	}

	return nil
}

// Interface guards.
var (
	_ Transport            = (*RedisTransport)(nil)
	_ TransportSubscribers = (*RedisTransport)(nil)
)
