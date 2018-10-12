package hub

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

func TestNewBoltFromEnv(t *testing.T) {
	db, _ := NewBoltFromEnv()
	defer os.Remove("updates.db")

	assert.FileExists(t, "updates.db")
	assert.IsType(t, &bolt.DB{}, db)

	os.Setenv("DB_PATH", "test.db")
	defer os.Unsetenv("DB_PATH")

	db, _ = NewBoltFromEnv()
	defer os.Remove("test.db")

	assert.FileExists(t, "test.db")
}

func TestBoltHistory(t *testing.T) {
	db, _ := bolt.Open("test.db", 0600, nil)
	defer db.Close()
	defer os.Remove("test.db")

	h := &BoltHistory{db}
	assert.Implements(t, (*History)(nil), h)

	count := 0
	assert.Nil(t, h.FindFor(&Subscriber{[]string{}, []*regexp.Regexp{}, ""}, func(*Update) bool {
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
		&Subscriber{
			[]string{"foo"},
			[]*regexp.Regexp{regexp.MustCompile(`^http:\/\/example\.com\/alt\/3$`)},
			"first",
		},
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

func TestNoHistory(t *testing.T) {
	h := &NoHistory{}
	assert.Nil(t, h.Add(nil))
	assert.Nil(t, h.FindFor(nil, func(*Update) bool { return true }))
}
