package hub

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"sync"

	bolt "go.etcd.io/bbolt"
	"go.uber.org/atomic"

	log "github.com/sirupsen/logrus"
)

const defaultBoltBucketName = "updates"

// BoltTransport implements the TransportInterface using the Bolt database
type BoltTransport struct {
	sync.RWMutex
	db               *bolt.DB
	bucketName       string
	size             uint64
	cleanupFrequency float64
	pipes            map[*Pipe]struct{}
	done             chan struct{}
	lastSeq          atomic.Uint64
}

// NewBoltTransport create a new BoltTransport
func NewBoltTransport(u *url.URL) (*BoltTransport, error) {
	var err error
	q := u.Query()
	bucketName := defaultBoltBucketName
	if q.Get("bucket_name") != "" {
		bucketName = q.Get("bucket_name")
	}

	size := uint64(0)
	if q.Get("size") != "" {
		size, err = strconv.ParseUint(q.Get("size"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf(`invalid bolt "%s" dsn: parameter size: %w`, u, err)
		}
	}

	cleanupFrequency := 0.3
	if q.Get("cleanup_frequency") != "" {
		cleanupFrequency, err = strconv.ParseFloat(q.Get("cleanup_frequency"), 64)
		if err != nil {
			return nil, fmt.Errorf(`invalid bolt "%s" dsn: parameter cleanup_frequency: %w`, u, err)
		}
	}

	path := u.Path // absolute path (bolt:///path.db)
	if path == "" {
		path = u.Host // relative path (bolt://path.db)
	}
	if path == "" {
		return nil, fmt.Errorf(`invalid bolt DSN "%s": missing path`, u)
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf(`invalid bolt DSN "%s": %w`, u, err)
	}

	return &BoltTransport{db: db, bucketName: bucketName, size: size, cleanupFrequency: cleanupFrequency, pipes: make(map[*Pipe]struct{}), done: make(chan struct{})}, nil
}

// Write pushes updates in the Transport
func (t *BoltTransport) Write(update *Update) error {
	select {
	case <-t.done:
		return ErrClosedTransport
	default:
	}

	t.RLock()

	if err := t.persist(update); err != nil {
		return err
	}

	var closedPipes []*Pipe
	for pipe := range t.pipes {
		if !pipe.Write(update) {
			closedPipes = append(closedPipes, pipe)
		}
	}

	t.RUnlock()
	t.Lock()

	for _, pipe := range closedPipes {
		delete(t.pipes, pipe)
	}

	t.Unlock()

	return nil
}

// persist stores update in the database
func (t *BoltTransport) persist(update *Update) error {
	return t.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(t.bucketName))
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
		t.lastSeq.Store(seq)
		prefix := make([]byte, 8)
		binary.BigEndian.PutUint64(prefix, seq)

		// The sequence value is prepended to the update id to create an ordered list
		key := bytes.Join([][]byte{prefix, []byte(update.ID)}, []byte{})

		if err := t.cleanup(bucket, seq); err != nil {
			return err
		}

		// The DB is append only
		bucket.FillPercent = 1
		return bucket.Put(key, buf)
	})
}

// CreatePipe returns a pipe fetching updates from the given point in time
func (t *BoltTransport) CreatePipe(fromID string) (*Pipe, error) {
	t.Lock()
	defer t.Unlock()

	select {
	case <-t.done:
		return nil, ErrClosedTransport
	default:
	}

	pipe := NewPipe()
	t.pipes[pipe] = struct{}{}
	if fromID == "" {
		return pipe, nil
	}

	toSeq := t.lastSeq.Load()
	go t.fetch(fromID, toSeq, pipe)

	return pipe, nil
}

func (t *BoltTransport) fetch(fromID string, toSeq uint64, pipe *Pipe) {
	err := t.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(t.bucketName))
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

			if !pipe.Write(update) || (toSeq > 0 && binary.BigEndian.Uint64(k[:8]) >= toSeq) {
				return nil
			}
		}

		return nil
	})
	if err != nil {
		log.Error(fmt.Errorf("bolt history: %w", err))
	}
}

// Close closes the Transport
func (t *BoltTransport) Close() error {
	select {
	case <-t.done:
		// Already closed. Don't close again.
	default:
		t.Lock()
		defer t.Unlock()
		for pipe := range t.pipes {
			pipe.Close()
		}
		close(t.done)
		t.db.Close()
	}

	return nil
}

// cleanup removes entries in the history above the size limit, triggered probabilistically
func (t *BoltTransport) cleanup(bucket *bolt.Bucket, lastID uint64) error {
	if t.size == 0 ||
		t.cleanupFrequency == 0 ||
		t.size >= lastID ||
		(t.cleanupFrequency != 1 && rand.Float64() < t.cleanupFrequency) {
		return nil
	}

	removeUntil := lastID - t.size
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
