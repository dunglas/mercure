package mercure

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const BoltDefaultCleanupFrequency = 0.3

const defaultBoltBucketName = "updates"

// maxHistoryScan is the default bound on the search for a requested
// Last-Event-ID: without it an ancient or forged id forces an O(history) scan
// on every request. See historyScanLimit.
const maxHistoryScan = 10000

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

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		// Path comes from operator config (Caddyfile or env), not HTTP input.
		if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec
			return nil, &TransportError{err: fmt.Errorf("creating bolt data directory %q: %w", dir, err)}
		}
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
		return "", fmt.Errorf("unable to get last_event_id from BoltDB: %w", err)
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

	// Marshal through the pointer so Update's custom MarshalJSON applies.
	updateJSON, err := json.Marshal(update)
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

	if s.RequestLastEventIDSet {
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

// pastSeqBound reports whether the BoltDB key k was written strictly after
// the sequence snapshot toSeq, and therefore falls outside the subscriber's
// history window. Events whose seq equals toSeq are the most recent ones
// observed at subscription time and are still considered part of history.
// toSeq == 0 means the bucket was empty at subscription time, so any key
// (all with seq >= 1) is "past the bound".
func pastSeqBound(k []byte, toSeq uint64) bool {
	return binary.BigEndian.Uint64(k[:8]) > toSeq
}

// findLastEventID returns the seq of the requested Last-Event-ID. Searching
// backwards matches how subscribers reconnect: forwards from the oldest event,
// scanLimit cut off exactly the recent ids they ask for.
func findLastEventID(b *bolt.Bucket, lastEventID string, toSeq, scanLimit uint64) (uint64, bool) {
	var scanned uint64

	c := b.Cursor()
	for k, _ := c.Last(); k != nil; k, _ = c.Prev() {
		if pastSeqBound(k, toSeq) {
			continue
		}

		if string(k[8:]) == lastEventID {
			return binary.BigEndian.Uint64(k[:8]), true
		}

		scanned++
		if scanned >= scanLimit {
			return 0, false
		}
	}

	return 0, false
}

// historyScanLimit bounds the search for a requested Last-Event-ID. A
// configured size is retention the operator pays for, so it stays searchable;
// maxHistoryScan is the floor and the only bound on an unlimited history.
func (t *BoltTransport) historyScanLimit() uint64 {
	if t.size > maxHistoryScan {
		return t.size
	}

	return maxHistoryScan
}

// replayHistory dispatches the authorized updates stored after fromSeq.
func (t *BoltTransport) replayHistory(
	ctx context.Context,
	s *LocalSubscriber,
	b *bolt.Bucket,
	fromSeq, toSeq uint64,
	responseLastEventID string,
) error {
	from := make([]byte, 8)
	binary.BigEndian.PutUint64(from, fromSeq+1)

	c := b.Cursor()
	for k, v := c.Seek(from); k != nil; k, v = c.Next() {
		// Dispatched since the subscribe snapshot: the live queue owns it.
		if pastSeqBound(k, toSeq) {
			break
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

		if s.Match(update) && !s.Dispatch(ctx, update, true) {
			break
		}
	}

	s.HistoryDispatched(responseLastEventID)

	return nil
}

func (t *BoltTransport) dispatchHistory(ctx context.Context, s *LocalSubscriber, toSeq uint64) error {
	ctx, span := startSpan(ctx, "mercure.transport.history",
		trace.WithAttributes(
			attribute.String("mercure.transport", "bolt"),
			attribute.String("mercure.subscriber.id", s.ID),
			attribute.String("mercure.last_event_id.requested", s.RequestLastEventID),
		))
	defer span.End()

	err := t.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(t.bucketName))
		if b == nil {
			s.HistoryDispatched(EarliestLastEventID)

			return nil // No data
		}

		if s.RequestLastEventID == EarliestLastEventID {
			return t.replayHistory(ctx, s, b, 0, toSeq, EarliestLastEventID)
		}

		fromSeq, found := findLastEventID(b, s.RequestLastEventID, toSeq, t.historyScanLimit())
		if !found {
			// Nothing replayed, so no event precedes a first one sent: the
			// protocol reserves "earliest" to tell the subscriber to re-fetch.
			// Unblock it before logging, it cannot send headers until then.
			s.HistoryDispatched(EarliestLastEventID)

			if t.logger.Enabled(ctx, slog.LevelInfo) {
				t.logger.LogAttrs(ctx, slog.LevelInfo, "Can't find requested LastEventID")
			}

			return nil
		}

		// The subscriber already knows this id; echoing it is not a disclosure.
		return t.replayHistory(ctx, s, b, fromSeq, toSeq, s.RequestLastEventID)
	})
	if err != nil {
		err = fmt.Errorf("unable to retrieve history from BoltDB: %w", err)
		recordSpanError(span, err)

		return err
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

// shouldCleanup reports whether this publish runs a cleanup pass. cleanupFrequency
// is the probability of doing so, from 0 (never) to 1 (every publish): drawing
// below it is what triggers the pass. There is nothing to remove while the
// history is still within the size limit.
func (t *BoltTransport) shouldCleanup(lastID uint64) bool {
	if t.size == 0 || t.size >= lastID {
		return false
	}

	return rand.Float64() < t.cleanupFrequency //nolint:gosec
}

// cleanup removes entries in the history above the size limit, triggered probabilistically.
func (t *BoltTransport) cleanup(bucket *bolt.Bucket, lastID uint64) error {
	if !t.shouldCleanup(lastID) {
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
