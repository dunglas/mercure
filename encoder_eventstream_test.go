package mercure

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventStreamEncoderContentType(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"text/event-stream"}, eventStreamEncoder{}.contentType())
}

// The preamble is a bare comment, written only to force the headers onto the
// wire, so it must carry no fields a subscriber could mistake for an update.
func TestEventStreamEncoderPreamble(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ":\n", eventStreamEncoder{}.preamble())
}

func TestEventStreamEncoderEncode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		u    *Update
		want string
	}{
		{
			"id and data",
			&Update{Event: Event{ID: "a", Data: "hi"}},
			"id: a\ndata: hi\n\n",
		},
		{
			// type and retry precede the id, and are omitted when unset.
			"every field",
			&Update{Event: Event{ID: "a", Data: "hi", Type: "update", Retry: 5}},
			"event: update\nretry: 5\nid: a\ndata: hi\n\n",
		},
		{
			// A data field carries one line each, per the event stream format.
			"data spanning several lines",
			&Update{Event: Event{ID: "a", Data: "one\ntwo"}},
			"id: a\ndata: one\ndata: two\n\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, eventStreamEncoder{}.encode(tc.u))
		})
	}
}

// The framing is Event.String(); encoding must not diverge from it.
func TestEventStreamEncoderEncodeMatchesEventString(t *testing.T) {
	t.Parallel()

	u := &Update{Event: Event{ID: "a", Data: "hi", Type: "update", Retry: 5}}

	assert.Equal(t, u.String(), eventStreamEncoder{}.encode(u))
}

// Topics are not part of an event, and never reach a subscriber. This pins
// that they stay that way.
func TestEventStreamEncoderOmitsTopics(t *testing.T) {
	t.Parallel()

	encoded := eventStreamEncoder{}.encode(&Update{
		Topics:  []string{"https://example.com/books/1", "https://example.com/users/42/books/1"},
		Private: true,
		Event:   Event{ID: "a", Data: "hi"},
	})

	assert.NotContains(t, encoded, "users/42")
	assert.NotContains(t, encoded, "topic")
	assert.NotContains(t, encoded, "private")
}

// A comment, which a subscriber ignores, keeping proxies and old browsers
// from closing an idle connection.
func TestEventStreamEncoderHeartbeat(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ":\n", eventStreamEncoder{}.heartbeat())
}

// The assembled stream is what a subscriber actually reads: a comment, then
// one event per update, each ending in a blank line.
func TestEventStreamEncoderStream(t *testing.T) {
	t.Parallel()

	e := eventStreamEncoder{}

	stream := e.preamble() +
		e.encode(&Update{Event: Event{ID: "a", Data: "first"}}) +
		e.heartbeat() +
		e.encode(&Update{Event: Event{ID: "b", Data: "second"}})

	assert.Equal(t, ":\nid: a\ndata: first\n\n:\nid: b\ndata: second\n\n", stream)

	// Every event is terminated, so a subscriber never waits on a partial one.
	assert.True(t, strings.HasSuffix(stream, "\n\n"))
}
