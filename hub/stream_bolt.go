package hub

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"

	bolt "go.etcd.io/bbolt"
	"go.uber.org/atomic"

	log "github.com/sirupsen/logrus"
)

const defaultBoltBucketName = "updates"

// BoltStream implements the StreamInterface using the Bolt database
type BoltStream struct {
	sync.RWMutex
	db               *bolt.DB
	bucketName       string
	size             uint64
	cleanupFrequency float64
	pipes            map[*Pipe]struct{}
	done             chan struct{}
	lastSeq          atomic.Uint64
}

// NewBoltStream create a new BoltStream
func NewBoltStream(options *Options) (*BoltStream, error) {
	var err error
	url := options.TransportURL
	q := url.Query()
	bucketName := defaultBoltBucketName
	if q.Get("bucket_name") != "" {
		bucketName = q.Get("bucket_name")
	}

	size := uint64(0)
	if q.Get("size") != "" {
		size, err = strconv.ParseUint(q.Get("size"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf(`invalid bolt "%s" dsn: parameter size: %w`, url, err)
		}
	}

	cleanupFrequency := 0.3
	if q.Get("cleanup_frequency") != "" {
		cleanupFrequency, err = strconv.ParseFloat(q.Get("cleanup_frequency"), 64)
		if err != nil {
			return nil, fmt.Errorf(`invalid bolt "%s" dsn: parameter cleanup_frequency: %w`, url, err)
		}
	}

	path := url.Path // absolute path (bolt:///path.db)
	if path == "" {
		path = url.Host // relative path (bolt://path.db)
	}
	if path == "" {
		return nil, fmt.Errorf(`invalid bolt "%s" dsn: missing path`, url)
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf(`invalid bolt "%s" dsn: %w`, url, err)
	}

	return &BoltStream{db: db, bucketName: bucketName, size: size, cleanupFrequency: cleanupFrequency, pipes: make(map[*Pipe]struct{}), done: make(chan struct{})}, nil
}

// Write pushes updates in the Stream
func (s *BoltStream) Write(update *Update) error {
	select {
	case <-s.done:
		return ErrClosedStream
	default:
	}

	var (
		err         error
		closedPipes []*Pipe
	)

	s.RLock()

	err = s.persist(update)

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

// persist stores update in the database
func (s *BoltStream) persist(update *Update) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(s.bucketName))
		if err != nil {
			return err
		}

		buf, err := json.Marshal(*update)
		if err != nil {
			return err
		}

		seq, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		s.lastSeq.Store(seq)
		prefix := make([]byte, 8)
		binary.BigEndian.PutUint64(prefix, seq)

		// The sequence value is prepended to the update id to create an ordered list
		key := bytes.Join([][]byte{prefix, []byte(update.ID)}, []byte{})

		if err := s.cleanup(bucket, seq); err != nil {
			return err
		}

		// The DB is append only
		bucket.FillPercent = 1
		return bucket.Put(key, buf)
	})
}

// CreatePipe returns a pipe fetching updates from the given point in time
func (s *BoltStream) CreatePipe(fromID string) (*Pipe, error) {
	s.Lock()
	defer s.Unlock()

	select {
	case <-s.done:
		return nil, ErrClosedStream
	default:
	}

	pipe := NewPipe()
	s.pipes[pipe] = struct{}{}
	if fromID == "" {
		return pipe, nil
	}

	toSeq := s.lastSeq.Load()
	go func() {
		err := s.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(s.bucketName))
			if b == nil {
				return nil // No data
			}

			c := b.Cursor()
			afterFromID := false
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if !afterFromID {
					if string(k[8:]) == fromID {
						afterFromID = true
					}

					continue
				}

				var update *Update
				if err := json.Unmarshal(v, &update); err != nil {
					return err
				}

				if !pipe.Write(update) {
					return nil
				}

				if toSeq > 0 && binary.BigEndian.Uint64(k[:8]) >= toSeq {
					return nil
				}
			}

			return nil
		})
		if err != nil {
			log.Error(fmt.Errorf("bolt history: %w", err))
		}
	}()

	return pipe, nil
}

// Close closes the Stream
func (s *BoltStream) Close() error {
	select {
	case <-s.done:
		// Already closed. Don't close again.
	default:
		s.RLock()
		defer s.RUnlock()
		for pipe := range s.pipes {
			pipe.Close()
		}
		close(s.done)
		s.db.Close()
	}

	return nil
}

// cleanup removes entries in the history above the size limit, triggered probabilistically
func (s *BoltStream) cleanup(bucket *bolt.Bucket, lastID uint64) error {
	if s.size == 0 ||
		s.cleanupFrequency == 0 ||
		s.size >= lastID ||
		(s.cleanupFrequency != 1 && rand.Float64() < s.cleanupFrequency) {
		return nil
	}

	removeUntil := lastID - s.size
	c := bucket.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		if binary.BigEndian.Uint64(k[:8]) > removeUntil {
			break
		}

		if err := bucket.Delete(k); err != nil {
			return err
		}
	}

	return nil
}
