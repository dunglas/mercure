package mercure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
)

const (
	lastEventIDKey = "lastEventID"
	publishScript  = `
		redis.call("SET", KEYS[1], ARGV[1])
		redis.call("PUBLISH", ARGV[2], ARGV[3])
		return true
	`
)

var errAuthFailed = errors.New("AUTH failed")

type RedisTransport struct {
	sync.RWMutex

	logger        Logger
	client        *redis.Client
	subscribers   *SubscriberList
	closed        chan any
	publishScript *redis.Script
	closedOnce    sync.Once
	redisChannel  string
}

// RedisConfig holds configuration for Redis connection.
type RedisConfig struct {
	Address         string
	Username        string
	Password        string
	SubscribersSize int
	RedisChannel    string
	// IAM authentication for Google Cloud Memorystore.
	UseIAMAuth bool
	ProjectID  string
	Location   string
	InstanceID string
}

func NewRedisTransport(
	logger Logger,
	address string,
	username string,
	password string,
	subscribersSize int,
	redisChannel string,
) (*RedisTransport, error) {
	config := &RedisConfig{
		Address:         address,
		Username:        username,
		Password:        password,
		SubscribersSize: subscribersSize,
		RedisChannel:    redisChannel,
		UseIAMAuth:      false,
	}

	return NewRedisTransportWithConfig(logger, config)
}

// NewRedisTransportWithIAM creates a Redis transport with IAM authentication for Google Cloud Memorystore.
func NewRedisTransportWithIAM(
	logger Logger,
	projectID string,
	location string,
	instanceID string,
	subscribersSize int,
	redisChannel string,
) (*RedisTransport, error) {
	// For Google Cloud Memorystore, the address should be the actual Redis endpoint
	// The format is typically: <instance-ip>:6379
	// We'll need to construct this properly or accept the full address
	config := &RedisConfig{
		Address:         fmt.Sprintf("%s:%s:%s", projectID, location, instanceID),
		SubscribersSize: subscribersSize,
		RedisChannel:    redisChannel,
		UseIAMAuth:      true,
		ProjectID:       projectID,
		Location:        location,
		InstanceID:      instanceID,
	}

	return NewRedisTransportWithConfig(logger, config)
}

// NewRedisTransportWithIAMAddress creates a Redis transport with IAM authentication using the full Memorystore address.
func NewRedisTransportWithIAMAddress(
	logger Logger,
	address string,
	projectID string,
	subscribersSize int,
	redisChannel string,
) (*RedisTransport, error) {
	config := &RedisConfig{
		Address:         address,
		SubscribersSize: subscribersSize,
		RedisChannel:    redisChannel,
		UseIAMAuth:      true,
		ProjectID:       projectID,
	}

	return NewRedisTransportWithConfig(logger, config)
}

// NewRedisTransportWithConfig creates a Redis transport with the given configuration.
func NewRedisTransportWithConfig(logger Logger, config *RedisConfig) (*RedisTransport, error) {
	var client *redis.Client

	var err error

	if config.UseIAMAuth {
		// Use IAM authentication for Google Cloud Memorystore
		client, err = createRedisClientWithIAM(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis client with IAM auth: %w", err)
		}
	} else {
		// Use traditional username/password authentication
		client = redis.NewClient(&redis.Options{
			Username: config.Username,
			Password: config.Password,
			Addr:     config.Address,
		})
	}

	if pong := client.Ping(context.Background()); pong.String() != "ping: PONG" {
		return nil, fmt.Errorf("failed to connect to Redis: %w", pong.Err())
	}

	// Log successful Redis connection
	logger.Info("Redis connection established",
		zap.String("address", config.Address),
		zap.Bool("useIAM", config.UseIAMAuth),
	)

	return NewRedisTransportInstance(logger, client, config.SubscribersSize, config.RedisChannel)
}

// createRedisClientWithIAM creates a Redis client with IAM authentication.
func createRedisClientWithIAM(config *RedisConfig) (*redis.Client, error) {
	ctx := context.Background()

	// The correct scope for Memorystore IAM is:
	// "https://www.googleapis.com/auth/cloud-platform"
	tokenSource, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to get default token source: %w", err)
	}

	// Create Redis client with IAM authentication
	client := redis.NewClient(&redis.Options{
		Addr: config.Address,
		// Use custom dialer for IAM authentication
		Dialer: func(_ context.Context, network, addr string) (net.Conn, error) {
			// Get the token
			token, err := tokenSource.Token()
			if err != nil {
				return nil, fmt.Errorf("failed to get token: %w", err)
			}

			// Create connection
			dialer := &net.Dialer{}
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, fmt.Errorf("failed to dial connection: %w", err)
			}

			// For Google Cloud Memorystore, we need to send the token in the AUTH command
			// The format is: AUTH <token>
			authCommand := fmt.Sprintf("AUTH %s\r\n", token.AccessToken)
			_, err = conn.Write([]byte(authCommand))
			if err != nil {
				conn.Close()

				return nil, fmt.Errorf("failed to send AUTH command: %w", err)
			}

			// Read response
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				conn.Close()

				return nil, fmt.Errorf("failed to read AUTH response: %w", err)
			}

			response := string(buf[:n])
			if response != "+OK\r\n" {
				conn.Close()

				return nil, fmt.Errorf("%w: %s", errAuthFailed, response)
			}

			return conn, nil
		},
	})

	return client, nil
}

func NewRedisTransportInstance(
	logger Logger,
	client *redis.Client,
	subscribersSize int,
	redisChannel string,
) (*RedisTransport, error) {
	subscriber := client.PSubscribe(context.Background(), redisChannel)

	subscribeCtx, subscribeCancel := context.WithCancel(context.Background())

	transport := &RedisTransport{
		logger:        logger,
		client:        client,
		subscribers:   NewSubscriberList(subscribersSize),
		publishScript: redis.NewScript(publishScript),
		closed:        make(chan any),
		redisChannel:  redisChannel,
	}

	go func() {
		select {
		case <-transport.closed:
			if err := subscriber.Close(); err != nil && !errors.Is(err, redis.ErrClosed) {
				logger.Error(err.Error())
			}

			<-subscribeCtx.Done()

			if err := client.Close(); err != nil && !errors.Is(err, redis.ErrClosed) {
				logger.Error(err.Error())
			}

			// Log Redis connection closure
			logger.Info("Redis connection closed",
				zap.String("address", transport.client.Options().Addr),
			)
		case <-subscribeCtx.Done():
		}
	}()

	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()

		transport.subscribe(subscribeCtx, subscribeCancel, subscriber)
	}()

	return transport, nil
}

func (u Update) MarshalBinary() ([]byte, error) {
	bytes, err := json.Marshal(u)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal: %w", err)
	}

	return bytes, nil
}

func (t *RedisTransport) Dispatch(update *Update) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	AssignUUID(update)

	keys := []string{lastEventIDKey}
	arguments := []interface{}{update.ID, t.redisChannel, update}

	_, err := t.publishScript.Run(context.Background(), t.client, keys, arguments...).Result()
	if err != nil {
		return fmt.Errorf("redis failed to publish: %w", err)
	}

	return nil
}

func (t *RedisTransport) AddSubscriber(s *LocalSubscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	t.subscribers.Add(s)
	t.Unlock()

	if s.RequestLastEventID != "" {
		s.HistoryDispatched(EarliestLastEventID)
	}

	s.Ready()

	return nil
}

func (t *RedisTransport) RemoveSubscriber(s *LocalSubscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	defer t.Unlock()

	t.subscribers.Remove(s)

	return nil
}

func (t *RedisTransport) GetSubscribers() (string, []*Subscriber, error) {
	select {
	case <-t.closed:
		return "", nil, ErrClosedTransport
	default:
	}

	t.RLock()
	defer t.RUnlock()

	lastEventID, err := t.client.Get(context.Background(), lastEventIDKey).Result()
	if err != nil {
		return "", nil, fmt.Errorf("redis failed to get last event id: %w", err)
	}

	return lastEventID, getSubscribers(t.subscribers), nil
}

func (t *RedisTransport) Close() (err error) {
	t.closedOnce.Do(func() {
		t.Lock()
		defer t.Unlock()

		// Log transport shutdown
		t.logger.Info("Redis transport shutting down",
			zap.String("address", t.client.Options().Addr),
		)

		t.subscribers.Walk(0, func(s *LocalSubscriber) bool {
			s.Disconnect()

			return true
		})

		close(t.closed)
	})

	return nil
}

func (t *RedisTransport) subscribe(ctx context.Context, cancel context.CancelFunc, subscriber *redis.PubSub) {
	for {
		message, err := subscriber.ReceiveMessage(ctx)
		if err != nil {
			if errors.Is(err, redis.ErrClosed) {
				cancel()

				return
			}

			t.logger.Error(err.Error())

			continue
		}

		var update Update

		if err := json.Unmarshal([]byte(message.Payload), &update); err != nil {
			t.logger.Error(err.Error())

			continue
		}

		topics := make([]string, len(update.Topics))
		copy(topics, update.Topics)

		t.Lock()

		for _, subscriber := range t.subscribers.MatchAny(&update) {
			update.Topics = topics
			subscriber.Dispatch(&update, false)
		}

		t.Unlock()
	}
}

var (
	_ Transport            = (*RedisTransport)(nil)
	_ TransportSubscribers = (*RedisTransport)(nil)
)
