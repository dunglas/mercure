package hub

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"regexp"

	bolt "go.etcd.io/bbolt"
)

// History stores and allows to retrieve the updates
// It is used to send previous messages when Last-Event-ID is provided by the subscriber
type History interface {
	// Add push an update in the history
	// Will return true in case of error (the update hasn't been stored)
	Add(*Update) error

	// Find retrieves updates pushed since the provided Last-Event-ID matching both the provided topics and targets
	// The onItem func will be called for every retrieved item, if its return value is false, Find will stop
	Find(lastEventID string, targets []string, topics []*regexp.Regexp, onItem func(*Update) bool) error
}

// NoHistory implements the History interface but does nothing
type NoHistory struct {
}

// Add does nothing
func (*NoHistory) Add(*Update) error {
	return nil
}

// Find does nothing
func (*NoHistory) Find(lastEventID string, targets []string, topics []*regexp.Regexp, onItem func(*Update) bool) error {
	return nil
}

const bucketName = "updates"

// BoltHistory is an implementation of the History interface using the Bolt DB
type BoltHistory struct {
	*bolt.DB
}

// NewBoltFromEnv opens the Bolt database, it finds the path in the DB_PATH env var
func NewBoltFromEnv() (*bolt.DB, error) {
	path := os.Getenv("DB_PATH")
	if path == "" {
		path = "updates.db"
	}

	return bolt.Open(path, 0600, nil)
}

// Add puts the update to the local bolt DB
func (b *BoltHistory) Add(update *Update) error {
	return b.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		buf, err := json.Marshal(*update)
		if err != nil {
			return err
		}

		s, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		prefix := make([]byte, 8)
		binary.BigEndian.PutUint64(prefix, s)

		// The sequence value is prepended to the update id to create an ordered list
		key := bytes.Join([][]byte{prefix, []byte(update.ID)}, []byte{})
		if err := bucket.Put(key, buf); err != nil {
			return err
		}

		return nil
	})
}

// Find searches in the local bolt DB
func (b *BoltHistory) Find(lastEventID string, targets []string, topics []*regexp.Regexp, onItem func(*Update) bool) error {
	b.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			// No data
			return nil
		}

		c := b.Cursor()
		afterLastEventID := false
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if !afterLastEventID {
				if string(k[8:]) == lastEventID {
					afterLastEventID = true
				}

				continue
			}

			var update Update
			if err := json.Unmarshal(v, &update); err != nil {
				return err
			}

			if CanDispatch(&update, targets, topics) && !onItem(&update) {
				return nil
			}
		}

		return nil
	})

	return nil
}
