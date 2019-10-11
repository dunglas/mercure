package hub

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

const defaultRedisStreamName = "mercure-hub-history"

func redisNilToNil(err error) error {
	if err == redis.Nil {
		return nil
	}
	return err
}

// RedisStream implements the StreamInterface using the Redis database
type RedisStream struct {
	sync.RWMutex
	client     *redis.Client
	streamName string
	streamSize int64
	pipes      map[*Pipe]struct{}
	done       chan struct{}
	lastSeq    atomic.String
}

// NewRedisStream create a new RedisStream
func NewRedisStream(options *Options) (*RedisStream, error) {
	var err error

	url := options.TransportURL
	q := url.Query()
	streamName := defaultRedisStreamName
	if q.Get("stream_name") != "" {
		streamName = q.Get("stream_name")
		q.Del("stream_name")
	}
	masterName := ""
	if q.Get("master_name") != "" {
		masterName = q.Get("master_name")
		q.Del("master_name")
	}

	streamSize := int64(0)
	if q.Get("size") != "" {
		streamSize, err = strconv.ParseInt(q.Get("size"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf(`invalid redis "%s" dsn: parameter size: %w`, url, err)
		}
		q.Del("size")
	}

	url.RawQuery = q.Encode()

	redisOptions, err := redis.ParseURL(url.String())
	if err != nil {
		return nil, fmt.Errorf(`invalid redis "%s" dsn: %w`, url, err)
	}

	var client *redis.Client
	if masterName != "" {
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    masterName,
			DB:            redisOptions.DB,
			Password:      redisOptions.Password,
			SentinelAddrs: []string{redisOptions.Addr},
		})
	} else {
		client = redis.NewClient(redisOptions)
	}

	if _, err := client.Ping().Result(); err != nil {
		return nil, fmt.Errorf(`redis connection "%s": %w`, url, err)
	}

	return &RedisStream{client: client, streamName: streamName, streamSize: streamSize, pipes: make(map[*Pipe]struct{}), done: make(chan struct{})}, nil
}

// Write pushes updates in the Stream
func (s *RedisStream) Write(update *Update) error {
	select {
	case <-s.done:
		return ErrClosedStream
	default:
	}

	buf, err := json.Marshal(*update)
	if err != nil {
		return err
	}

	var script string
	if s.streamSize > 0 {
		// this maintain a key(update.ID)/value(redis.Seq) index that where key may not be unique and number of item is the stream is limited
		script = `
			local limit = tonumber(ARGV[1])
			local streamID=redis.call("XADD", KEYS[1], "*", "MAXLEN", ARGV[1], "data", ARGV[2])
			redis.call("RPUSH", KEYS[2], streamID)
			redis.call("RPUSH", KEYS[3], KEYS[2])
			while (redis.call("LLEN", KEYS[3]) > limit) do
				local key = redis.call("LPOP", KEYS[3])
				redis.call("LPOP", key)
				if redis.call("LLEN", key) == 0 then
					redis.call("DEL", key)
				end
			end`
	} else {
		script = `
			local streamID=redis.call("XADD", KEYS[1], "*", "data", ARGV[2])
			redis.call("RPUSH", KEYS[2], streamID)`
	}

	if err = s.client.Eval(script, []string{s.streamName, s.cacheKeyID(update.ID), s.cacheKeyID("")}, s.streamSize, buf).Err(); err != nil {
		return redisNilToNil(err)
	}

	return nil
}

// CreatePipe returns a pipe fetching updates from the given point in time
func (s *RedisStream) CreatePipe(fromID string) (*Pipe, error) {
	s.Lock()
	defer s.Unlock()

	select {
	case <-s.done:
		return nil, ErrClosedStream
	default:
	}

	pipe := NewPipe()
	toSeq, err := s.registerPipe(pipe)
	if err != nil {
		return nil, err
	}

	if fromID != "" && toSeq != "$" {
		go func() {
			err := func() error {
				res, err := s.client.LIndex(s.cacheKeyID(fromID), 0).Result()
				if err != nil {
					return err
				}

				messages, err := s.client.XRange(s.streamName, res, toSeq).Result()
				if err != nil {
					return err
				}

				first := true
				for _, entry := range messages {
					if first {
						first = false
						continue
					}
					message, ok := entry.Values["data"]
					if !ok {
						return fmt.Errorf(`stream return an invalid entry: %v`, entry.Values)
					}

					var update *Update
					if err := json.Unmarshal([]byte(fmt.Sprintf("%v", message)), &update); err != nil {
						return err
					}

					if !pipe.Write(update) {
						return nil
					}
				}

				return nil
			}()
			if err != nil {
				log.Error(fmt.Errorf("redis history: %w", err))
			}
		}()
	}

	return pipe, nil
}

// Close closes the Stream
func (s *RedisStream) Close() error {
	select {
	case <-s.done:
		// Already closed. Don't close again.
	default:
		close(s.done)
		s.RLock()
		defer s.RUnlock()
		for pipe := range s.pipes {
			pipe.Close()
		}
	}

	return nil
}

// cacheKeyID provides a unique cache identifier for the given ID
func (s *RedisStream) cacheKeyID(ID string) string {
	return fmt.Sprintf("%s/%s", s.streamName, ID)
}

func (s *RedisStream) fetchLastSequence() (string, error) {
	messages, err := s.client.XRevRangeN(s.streamName, "+", "-", 1).Result()
	if err != nil {
		return "", redisNilToNil(err)
	}

	for _, entry := range messages {
		return entry.ID, nil
	}

	return "", nil
}

func (s *RedisStream) registerPipe(pipe *Pipe) (string, error) {
	if s.lastSeq.Load() == "" {
		lastSeq, err := s.fetchLastSequence()
		if err != nil {
			return "", err
		}

		if lastSeq == "" {
			lastSeq = "$"
		}
		s.lastSeq.Store(lastSeq)

		go s.listenMessages()
	}

	s.pipes[pipe] = struct{}{}

	return s.lastSeq.Load(), nil
}

func (s *RedisStream) listenMessages() {
	for {
		select {
		case <-s.done:
			return
		default:
		}

		if err := s.readMessages(); err != nil {
			log.Error(err)
			time.Sleep(20 * time.Millisecond) // invoid infinite loop consuming CPU
		}
	}
}

func (s *RedisStream) readMessages() error {
	cmd := redis.NewXStreamSliceCmd("XREAD", "COUNT", "1", "BLOCK", int64(time.Second/time.Millisecond), "STREAMS", s.streamName, s.lastSeq.Load())
	s.client.Process(cmd)
	streams, err := cmd.Result()

	if err != nil {
		return redisNilToNil(err)
	}

	for _, stream := range streams {
		for _, entry := range stream.Messages {
			if err := s.processMessage(entry); err != nil {
				return err
			}
			s.lastSeq.Store(entry.ID)
		}
	}
	return nil
}

func (s *RedisStream) processMessage(entry redis.XMessage) error {
	message, ok := entry.Values["data"]
	if !ok {
		return fmt.Errorf(`stream returns an invalid entry: %v`, entry.Values)
	}

	var update *Update
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", message)), &update); err != nil {
		return err
	}

	var (
		err         error
		closedPipes []*Pipe
	)

	s.RLock()

	for pipe := range s.pipes {
		if !pipe.Write(update) {
			closedPipes = append(closedPipes, pipe)
		}
	}

	s.RUnlock()
	s.Lock()

	for _, pipe := range closedPipes {
		delete(s.pipes, pipe)
	}

	s.Unlock()

	return err
}
