package mercure

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

const BoltDefaultCleanupFrequency = 0.3

const defaultBoltBucketName = "updates"

// BoltTransport implements the TransportInterface using the Bolt database.
type BoltTransport struct {
	sync.RWMutex

	subscribers      *SubscriberList
	logger           *slog.Logger
	db               *bolt.DB
	bucketName       string
	size             uint64
	cleanupFrequency float64
	closed           chan struct{}
	closedOnce       sync.Once
	lastSeq          uint64
	lastEventID      string
}

// NewBoltTransport creates a new BoltTransport.
func NewBoltTransport(
	subscriberList *SubscriberList,
	logger *slog.Logger,
	path string,
	bucketName string,
	size uint64,
	cleanupFrequency float64,
) (*BoltTransport, error) {
	if path == "" {
		path = "bolt.db"
	}

	if bucketName == "" {
		bucketName = defaultBoltBucketName
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, &TransportError{err: err}
	}

	lastEventID, err := getDBLastEventID(db, bucketName)
	if err != nil {
		return nil, &TransportError{err: err}
	}

	return &BoltTransport{
		logger:           logger,
		db:               db,
		bucketName:       bucketName,
		size:             size,
		cleanupFrequency: cleanupFrequency,
		subscribers:      subscriberList,
		closed:           make(chan struct{}),
		lastEventID:      lastEventID,
	}, nil
}

func getDBLastEventID(db *bolt.DB, bucketName string) (string, error) {
	lastEventID := EarliestLastEventID

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil // No data
		}

		if k, _ := b.Cursor().Last(); k != nil {
			lastEventID = string(k[8:])
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("unable to get lastEventID from BoltDB: %w", err)
	}

	return lastEventID, nil
}

// Dispatch dispatches an update to all subscribers and persists it in Bolt DB.
func (t *BoltTransport) Dispatch(ctx context.Context, update *Update) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	update.AssignUUID()

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
		s.Dispatch(ctx, update, false)
	}

	return nil
}

// AddSubscriber adds a new subscriber to the transport.
func (t *BoltTransport) AddSubscriber(ctx context.Context, s *LocalSubscriber) error {
	select {
	case <-t.closed:
		return ErrClosedTransport
	default:
	}

	t.Lock()
	t.subscribers.Add(s)
	toSeq := t.lastSeq
	t.Unlock()

	if s.RequestLastEventID != "" {
		if err := t.dispatchHistory(ctx, s, toSeq); err != nil {
			return err
		}
	}

	s.Ready(ctx)

	return nil
}

// RemoveSubscriber removes a new subscriber from the transport.
func (t *BoltTransport) RemoveSubscriber(_ context.Context, s *LocalSubscriber) error {
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
func (t *BoltTransport) GetSubscribers(_ context.Context) (string, []*Subscriber, error) {
	t.RLock()
	defer t.RUnlock()

	return t.lastEventID, getSubscribers(t.subscribers), nil
}

// Close closes the Transport.
func (t *BoltTransport) Close(_ context.Context) (err error) {
	t.closedOnce.Do(func() {
		close(t.closed)

		t.Lock()
		defer t.Unlock()

		t.subscribers.Walk(0, func(s *LocalSubscriber) bool {
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

//nolint:gocognit
func (t *BoltTransport) dispatchHistory(ctx context.Context, s *LocalSubscriber, toSeq uint64) error {
	err := t.db.View(func(tx *bolt.Tx) error {
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

				err := fmt.Errorf("unable to unmarshal update: %w", err)

				if t.logger.Enabled(ctx, slog.LevelError) {
					t.logger.LogAttrs(ctx, slog.LevelError, "Unable to unmarshal update coming from the Bolt DB", slog.Any("update", update), slog.Any("error", err))
				}

				return err
			}

			if (s.Match(update) && !s.Dispatch(ctx, update, true)) || (toSeq > 0 && binary.BigEndian.Uint64(k[:8]) >= toSeq) {
				s.HistoryDispatched(responseLastEventID)

				return nil
			}
		}

		s.HistoryDispatched(responseLastEventID)

		if !afterFromID {
			if t.logger.Enabled(ctx, slog.LevelInfo) {
				t.logger.LogAttrs(ctx, slog.LevelInfo, "Can't find requested LastEventID")
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to retrieve history from BoltDB: %w", err)
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

		// The DB is append-only
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
