package hub

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yosida95/uritemplate"
	bolt "go.etcd.io/bbolt"
)

func TestBoltHistory(t *testing.T) {
	db, _ := bolt.Open("test.db", 0600, nil)
	defer db.Close()
	defer os.Remove("test.db")

	h := &boltHistory{db, &Options{}}
	assert.Implements(t, (*History)(nil), h)

	count := 0
	assert.Nil(t, h.FindFor(
		NewSubscriber(false, map[string]struct{}{}, []string{}, []*uritemplate.Template{}, ""),
		func(*Update) bool {
			count++
			return true
		}))
	assert.Equal(t, 0, count)

	assert.Nil(t, h.Add(&Update{Event: Event{ID: "first"}}))
	assert.Nil(t, h.Add(&Update{
		Targets: map[string]struct{}{"foo": {}},
		Topics:  []string{"http://example.com/2"},
		Event:   Event{ID: "second"},
	}))
	assert.Nil(t, h.Add(&Update{
		Targets: map[string]struct{}{"foo": {}, "bar": {}},
		Topics:  []string{"http://example.com/3", "http://example.com/alt/3"},
		Event:   Event{ID: "third", Data: "an update"},
	}))
	assert.Nil(t, h.Add(&Update{
		Event:   Event{ID: "fourth"},
		Topics:  []string{"http://example.com/alt/3"},
		Targets: map[string]struct{}{"baz": {}},
	}))
	assert.Nil(t, h.Add(&Update{
		Targets: map[string]struct{}{"foo": {}, "bar": {}},
		Topics:  []string{"http://example.com/alt/3"},
		Event:   Event{Data: "stop now"},
	}))
	assert.Nil(t, h.Add(&Update{
		Targets: map[string]struct{}{"foo": {}, "bar": {}},
		Topics:  []string{"http://example.com/alt/3"},
		Event:   Event{Data: "should not be called"},
	}))

	h.FindFor(
		NewSubscriber(false, map[string]struct{}{"foo": {}}, []string{"http://example.com/alt/3"}, []*uritemplate.Template{}, "first"),
		func(u *Update) bool {
			count++

			switch count {
			case 1:
				assert.Equal(t, "an update", u.Data)
				break

			case 2:
				assert.Equal(t, "stop now", u.Data)
				return false
			}

			return true
		},
	)

	assert.Equal(t, 2, count)
}

func TestPurgeHistory(t *testing.T) {
	db, _ := bolt.Open("test.db", 0600, nil)
	defer db.Close()
	defer os.Remove("test.db")

	o := &Options{HistorySize: 5, HistoryCleanupFrequency: 1}
	h := &boltHistory{db, o}

	for i := 0; i < 12; i++ {
		h.Add(&Update{Event: Event{ID: strconv.Itoa(i)}})
	}

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))

		assert.Equal(t, 5, b.Stats().KeyN)

		return nil
	})
}

func TestNoHistory(t *testing.T) {
	h := &noHistory{}
	assert.Nil(t, h.Add(nil))
	assert.Nil(t, h.FindFor(nil, func(*Update) bool { return true }))
}
