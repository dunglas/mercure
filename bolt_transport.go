package mercure

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const BoltDefaultCleanupFrequency = 0.3

func init() { //nolint:gochecknoinits
	RegisterTransportFactory("bolt", NewBoltTransport)
}

const defaultBoltBucketName = "updates"

// BoltTransport implements the TransportInterface using the Bolt database.
type BoltTransport struct {
	sync.RWMutex
	subscribers      *SubscriberList
	logger           Logger
	db               *bolt.DB
	bucketName       string
	size             uint64
	cleanupFrequency float64
	closed           chan struct{}
	closedOnce       sync.Once
	lastSeq          uint64
	lastEventID      string
}

// NewBoltTransport create a new boltTransport.
func NewBoltTransport(u *url.URL, l Logger) (Transport, error) { //nolint:ireturn
	var err error
	q := u.Query()
	bucketName := defaultBoltBucketName
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

	cleanupFrequency := BoltDefaultCleanupFrequency
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

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, &TransportError{dsn: u.Redacted(), err: err}
	}

	return &BoltTransport{
		logger:           l,
		db:               db,
		bucketName:       bucketName,
		size:             size,
		cleanupFrequency: cleanupFrequency,
		subscribers:      NewSubscriberList(1e5),
		closed:           make(chan struct{}),
		lastEventID:      getDBLastEventID(db, bucketName),
	}, nil
}

func getDBLastEventID(db *bolt.DB, bucketName string) string {
	lastEventID := EarliestLastEventID
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil // No data
		}

		if k, _ := b.Cursor().Last(); k != nil {
			lastEventID = string(k[8:])
		}

		return nil
	})

	return lastEventID
}

// Dispatch dispatches an update to all subscribers and persists it in Bolt DB.
func (t *BoltTransport) Dispatch(update *Update) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	AssignUUID(update)
	updateJSON, err := json.Marshal(*update)
	if err != nil {
		return fmt.Errorf("error when marshaling update: %w", err)
	}

	// We cannot use RLock() because Bolt allows only one read-write transaction at a time
	t.Lock()
	defer t.Unlock()

	if err := t.persist(update.ID, updateJSON); err != nil {
		return err
	}

	for _, s := range t.subscribers.MatchAny(update) {
		s.Dispatch(update, false)
	}

	return nil
}

// persist stores update in the database.
func (t *BoltTransport) persist(updateID string, updateJSON []byte) error {
	if err := t.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(t.bucketName))
		if err != nil {
			return fmt.Errorf("error when creating Bolt DB bucket: %w", err)
		}

		seq, err := bucket.NextSequence()
		if err != nil {
			return fmt.Errorf("error when generating Bolt DB sequence: %w", err)
		}
		prefix := make([]byte, 8)
		binary.BigEndian.PutUint64(prefix, seq)

		// The sequence value is prepended to the update id to create an ordered list
		key := bytes.Join([][]byte{prefix, []byte(updateID)}, []byte{})

		// The DB is append only
		bucket.FillPercent = 1

		t.lastSeq = seq
		t.lastEventID = updateID
		if err := bucket.Put(key, updateJSON); err != nil {
			return fmt.Errorf("unable to put value in Bolt DB: %w", err)
		}

		return t.cleanup(bucket, seq)
	}); err != nil {
		return fmt.Errorf("bolt error: %w", err)
	}

	return nil
}

// AddSubscriber adds a new subscriber to the transport.
func (t *BoltTransport) AddSubscriber(s *Subscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	t.subscribers.Add(s)
	toSeq := t.lastSeq //nolint:ifshort
	t.Unlock()

	if s.RequestLastEventID != "" {
		t.dispatchHistory(s, toSeq)
	}

	s.Ready()

	return nil
}

// RemoveSubscriber removes a new subscriber from the transport.
func (t *BoltTransport) RemoveSubscriber(s *Subscriber) error {
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

// GetSubscribers get the list of active subscribers.
func (t *BoltTransport) GetSubscribers() (string, []*Subscriber, error) {
	t.RLock()
	defer t.RUnlock()

	return t.lastEventID, getSubscribers(t.subscribers), nil
}

//nolint:gocognit
func (t *BoltTransport) dispatchHistory(s *Subscriber, toSeq uint64) {
	t.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(t.bucketName))
		if b == nil {
			s.HistoryDispatched(EarliestLastEventID)

			return nil // No data
		}

		c := b.Cursor()
		responseLastEventID := EarliestLastEventID
		afterFromID := s.RequestLastEventID == EarliestLastEventID
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if !afterFromID {
				responseLastEventID = string(k[8:])
				if responseLastEventID == s.RequestLastEventID {
					afterFromID = true
				}

				continue
			}

			var update *Update
			if err := json.Unmarshal(v, &update); err != nil {
				s.HistoryDispatched(responseLastEventID)
				if c := t.logger.Check(zap.ErrorLevel, "Unable to unmarshal update coming from the Bolt DB"); c != nil {
					c.Write(zap.Error(err))
				}

				return fmt.Errorf("unable to unmarshal update: %w", err)
			}

			if (s.Match(update) && !s.Dispatch(update, true)) || (toSeq > 0 && binary.BigEndian.Uint64(k[:8]) >= toSeq) {
				s.HistoryDispatched(responseLastEventID)

				return nil
			}
		}
		s.HistoryDispatched(responseLastEventID)
		if !afterFromID {
			if c := t.logger.Check(zap.InfoLevel, "Can't find requested LastEventID"); c != nil {
				c.Write(zap.String("LastEventID", s.RequestLastEventID))
			}
		}

		return nil
	})
}

// Close closes the Transport.
func (t *BoltTransport) Close() (err error) {
	t.closedOnce.Do(func() {
		close(t.closed)

		t.Lock()
		defer t.Unlock()

		t.subscribers.Walk(0, func(s *Subscriber) bool {
			s.Disconnect()

			return true
		})
		err = t.db.Close()
	})

	if err == nil {
		return nil
	}

	return fmt.Errorf("unable to close Bolt DB: %w", err)
}

// cleanup removes entries in the history above the size limit, triggered probabilistically.
func (t *BoltTransport) cleanup(bucket *bolt.Bucket, lastID uint64) error {
	if t.size == 0 ||
		t.cleanupFrequency == 0 ||
		t.size >= lastID ||
		(t.cleanupFrequency != 1 && rand.Float64() < t.cleanupFrequency) { //nolint:gosec
		return nil
	}

	removeUntil := lastID - t.size
	c := bucket.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		if binary.BigEndian.Uint64(k[:8]) > removeUntil {
			break
		}

		if err := bucket.Delete(k); err != nil {
			return fmt.Errorf("unable to delete value in Bolt DB: %w", err)
		}
	}

	return nil
}

// Interface guards.
var (
	_ Transport            = (*BoltTransport)(nil)
	_ TransportSubscribers = (*BoltTransport)(nil)
)
