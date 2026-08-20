package mercure

import (
	"bufio"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultipartDigestEncoderContentType(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	ct := e.contentType()
	require.Len(t, ct, 1)

	mediaType, params, err := mime.ParseMediaType(ct[0])
	require.NoError(t, err)
	assert.Equal(t, "multipart/digest", mediaType)
	assert.Equal(t, e.boundary, params["boundary"])
}

// Two encoders must not share a boundary: a payload that happened to contain
// one would otherwise confuse every connection alike.
func TestMultipartDigestEncoderBoundaryIsRandom(t *testing.T) {
	t.Parallel()

	a := newDigestEncoder(t)

	b := newDigestEncoder(t)

	assert.NotEqual(t, a.boundary, b.boundary)
}

// 16 hyphens and 16 random hex characters, the shape many browsers and HTTP
// libraries write.
func TestMultipartDigestEncoderBoundaryShape(t *testing.T) {
	t.Parallel()

	boundary := newDigestEncoder(t).boundary

	assert.Len(t, boundary, 32)
	assert.Equal(t, strings.Repeat("-", 16), boundary[:16])
	assert.Regexp(t, "^[0-9a-f]{16}$", boundary[16:])
}

func TestMultipartDigestEncoderEncode(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	assert.Equal(t,
		"\r\n\r\n"+
			"Event-ID: urn:1\r\n"+
			"Content-Type: text/plain; charset=utf-8\r\n"+
			"Content-Length: 2\r\n"+
			"\r\n"+
			"hi"+
			"\r\n--"+e.boundary,
		e.encode(&Update{Event: Event{ID: "urn:1", Data: "hi"}}),
	)
}

// type and retry are omitted when unset, as Event.String() omits the
// corresponding SSE fields.
func TestMultipartDigestEncoderOmitsUnsetFields(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	header, _ := encodedNotification(t, e, e.encode(&Update{Event: Event{ID: "a", Data: "d"}}))
	assert.Equal(t, "a", header.Get("Event-ID"))
	assert.Empty(t, header.Get("Event-Type"))
	assert.Empty(t, header.Get("Retry"))

	header, _ = encodedNotification(t, e,
		e.encode(&Update{Event: Event{ID: "a", Data: "d", Type: "t", Retry: 5}}))
	assert.Equal(t, "t", header.Get("Event-Type"))
	assert.Equal(t, "5", header.Get("Retry"))
}

// Topics never reach subscribers over Event Stream; an alternate topic can
// name another subscriber's namespace, so it must not leak here either.
func TestMultipartDigestEncoderOmitsTopics(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	part := e.encode(&Update{
		Topics:  []string{"https://example.com/books/1", "https://example.com/users/42/books/1"},
		Private: true,
		Event:   Event{ID: "a", Data: "d"},
	})

	assert.NotContains(t, part, "users/42")
	assert.NotContains(t, part, "Topic")
	assert.NotContains(t, part, "Private")
}

// Data may hold any valid UTF-8, including the control characters that
// Update.Validate permits. The message body is delimited by its length, so
// they travel as they were published.
func TestMultipartDigestEncoderCarriesDataVerbatim(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	const data = "line\r\nnext\x00null&amp=x"

	_, body := encodedNotification(t, e, e.encode(&Update{Event: Event{ID: "a", Data: data}}))
	assert.Equal(t, data, body)
}

// A payload containing the boundary must not terminate the part early. Every
// part declares a Content-Length, so a conforming parser reads the body by
// length rather than scanning for the delimiter.
func TestMultipartDigestEncoderBoundaryInPayload(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	data := "--" + e.boundary + "--"

	_, body := encodedNotification(t, e, e.encode(&Update{Event: Event{ID: "a", Data: data}}))
	assert.Equal(t, data, body)
}

// The keep-alive is a part carrying the smallest message there is: no fields
// and no body, one linebreak in all.
func TestMultipartDigestEncoderHeartbeatIsEmptyPart(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	assert.Equal(t, "\r\n\r\n\r\n\r\n--"+e.boundary, e.heartbeat())
}

// The whole stream must parse as a multipart body: preamble, parts, trailer.
func TestMultipartDigestEncoderStreamIsWellFormed(t *testing.T) {
	t.Parallel()

	e := newDigestEncoder(t)

	stream := e.preamble() +
		e.encode(&Update{Event: Event{ID: "a", Data: "first"}}) +
		e.heartbeat() +
		e.encode(&Update{Event: Event{ID: "b", Data: "second", Type: "update"}}) +
		e.trailer()

	r := multipart.NewReader(strings.NewReader(stream), e.boundary)

	type part struct {
		contentType string
		body        string
	}

	var parts []part

	for {
		p, err := r.NextPart()
		if err != nil {
			break
		}

		body, err := io.ReadAll(p)
		require.NoError(t, err)

		parts = append(parts, part{p.Header.Get("Content-Type"), string(body)})
	}

	require.Len(t, parts, 3, "expected two notifications and one keep-alive")

	assert.Empty(t, parts[0].contentType, "a notification takes the carrier's default")
	assert.Equal(t,
		"Event-ID: a\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: 5\r\n\r\nfirst",
		parts[0].body,
	)

	assert.Empty(t, parts[1].contentType, "the keep-alive is a message too")
	assert.Equal(t, "\r\n", parts[1].body)

	assert.Equal(t,
		"Event-ID: b\r\nEvent-Type: update\r\n"+
			"Content-Type: text/plain; charset=utf-8\r\nContent-Length: 6\r\n\r\nsecond",
		parts[2].body,
	)
}

// encodedNotification reads back one encoded part as the notification it
// carries, by the length the message declares rather than the delimiter,
// which a payload may contain.
func encodedNotification(t *testing.T, e *multipartDigestEncoder, part string) (textproto.MIMEHeader, string) {
	t.Helper()

	message, found := strings.CutPrefix(part, "\r\n\r\n")
	require.True(t, found, "a notification part carries no header of its own")

	r := bufio.NewReader(strings.NewReader(message))

	header, data := readNotification(t, r)

	rest, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, "\r\n--"+e.boundary, string(rest),
		"Content-Length disagrees with the delimiter closing the part")

	return header, data
}

// readNotification parses a notification: the message's header fields, then
// the data its Content-Length delimits.
func readNotification(t *testing.T, r *bufio.Reader) (textproto.MIMEHeader, string) {
	t.Helper()

	m := textproto.NewReader(r)

	header, err := m.ReadMIMEHeader()
	require.NoError(t, err)

	length, err := strconv.Atoi(header.Get("Content-Length"))
	require.NoError(t, err)

	data := make([]byte, length)
	_, err = io.ReadFull(m.R, data)
	require.NoError(t, err)

	return header, string(data)
}

// newDigestEncoder builds an encoder through its constructor, as the response
// side does, and hands back the concrete type whose boundary the assertions
// read.
func newDigestEncoder(t *testing.T) *multipartDigestEncoder {
	t.Helper()

	e, ok := newMultipartDigestEncoder().(*multipartDigestEncoder)
	require.True(t, ok)

	return e
}
