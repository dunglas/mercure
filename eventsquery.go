package mercure

import (
	"bytes"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/munnerz/goautoneg"
)

// Media types negotiable on subscription responses. text/event-stream is the
// default and always supported; multipart/mixed is offered on QUERY requests
// when the events query support is enabled (draft-gupta-httpapi-events-query).
const (
	mediaTypeEventStream = "text/event-stream"
	mediaTypeMultipart   = "multipart/mixed"
)

// Sent on every subscription response, whatever the encoding.
//
//nolint:gochecknoglobals
var (
	// Incremental (draft-ietf-httpbis-incremental) tells intermediaries to
	// forward each chunk immediately instead of buffering the response — the
	// standardized equivalent of X-Accel-Buffering: no, which SSE needs too.
	headerIncremental = []string{"?1"}
	// Accept-Query advertises the media type of the QUERY request body
	// (RFC 10008): the subscription parameters, form-encoded as for GET.
	// The hub accepts QUERY unconditionally, so this is not gated on the
	// events query support.
	headerAcceptQuery = []string{"application/x-www-form-urlencoded"}
)

// streamEncoder encodes the subscription response body in the negotiated
// media type. One instance per connection: encoders may hold framing state
// such as the multipart boundary.
type streamEncoder interface {
	// contentType is the response Content-Type value, including parameters.
	contentType() string
	// preamble opens the stream and flushes the headers; nil means
	// WriteHeader is called instead.
	preamble() []byte
	// encode serializes one update as a single write.
	encode(u *Update) (string, error)
	// heartbeat is the keep-alive payload; nil disables the heartbeat timer.
	heartbeat() []byte
	// trailer closes the stream on clean disconnection; nil means none.
	trailer() []byte
}

// sseEncoder is the default text/event-stream encoding.
type sseEncoder struct{}

func (sseEncoder) contentType() string { return mediaTypeEventStream }

// The comment is the only way to flush the headers without writing an event.
func (sseEncoder) preamble() []byte { return []byte{':', '\n'} }

func (sseEncoder) encode(u *Update) (string, error) { return u.String(), nil }

// An SSE comment as a heartbeat prevents issues with some proxies and old browsers.
func (sseEncoder) heartbeat() []byte { return []byte{':', '\n'} }

func (sseEncoder) trailer() []byte { return nil }

// multipartEncoder encodes each update as a multipart/mixed body part: the
// raw data as the part body, the event ID in the Content-ID header and the
// publisher-provided media type, if any, in Content-Type. SSE-specific
// properties (type, retry) have no equivalent in this encoding. The writer is
// bound to an internal buffer, never to the ResponseWriter, so each event
// still reaches the client as a single deadline-bounded write.
type multipartEncoder struct {
	buf bytes.Buffer
	mw  *multipart.Writer
}

func newMultipartEncoder() *multipartEncoder {
	e := &multipartEncoder{}
	// The random 60-hex-character boundary makes a collision with update data
	// practically impossible.
	e.mw = multipart.NewWriter(&e.buf)

	return e
}

func (e *multipartEncoder) contentType() string {
	return mediaTypeMultipart + "; boundary=" + e.mw.Boundary()
}

func (e *multipartEncoder) preamble() []byte { return nil }

func (e *multipartEncoder) encode(u *Update) (string, error) {
	e.buf.Reset()

	// RFC 2046 only allows Content-* fields in body part headers; the event
	// ID fits the Content-ID slot (RFC 2045, msg-id syntax).
	header := textproto.MIMEHeader{
		"Content-Id":     {"<" + u.ID + ">"},
		"Content-Length": {strconv.Itoa(len(u.Data))},
	}
	// The hub treats update data as opaque, so a media type is only asserted
	// when the publisher provided one.
	if u.ContentType != "" {
		header.Set("Content-Type", u.ContentType)
	}

	pw, err := e.mw.CreatePart(header)
	if err != nil {
		return "", fmt.Errorf("creating multipart event part: %w", err)
	}

	if _, err := pw.Write([]byte(u.Data)); err != nil {
		return "", fmt.Errorf("writing multipart event part: %w", err)
	}

	return e.buf.String(), nil
}

func (e *multipartEncoder) heartbeat() []byte { return nil }

func (e *multipartEncoder) trailer() []byte {
	e.buf.Reset()

	if err := e.mw.Close(); err != nil {
		return nil
	}

	return bytes.Clone(e.buf.Bytes())
}

// negotiateStreamEncoder selects the response encoding. text/event-stream is
// always the default: only an enabled events query support combined with a
// QUERY request whose Accept header prefers multipart/mixed switches the
// encoding, so enabling the feature cannot regress an existing client.
//
//nolint:ireturn // the encoder is chosen at negotiation time by design
func (h *Hub) negotiateStreamEncoder(r *http.Request) streamEncoder {
	if !h.eventsQuery || r.Method != methodQuery {
		return sseEncoder{}
	}

	if goautoneg.Negotiate(r.Header.Get("Accept"), []string{mediaTypeEventStream, mediaTypeMultipart}) == mediaTypeMultipart {
		return newMultipartEncoder()
	}

	return sseEncoder{}
}

// parseEventsDuration extracts the advisory "duration" member (in seconds)
// from an Events header field value, a Structured Fields Dictionary
// (RFC 9651). The member is a client preference the hub may ignore, so a
// minimal parser beats a structured-fields dependency: unparsable or
// non-positive values (0 means "no preference") report false and are ignored.
func parseEventsDuration(v string) (time.Duration, bool) {
	for member := range strings.SplitSeq(v, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(member), "=")
		if !found || key != "duration" {
			continue
		}

		// Drop any parameters attached to the member.
		value, _, _ = strings.Cut(value, ";")

		// The value is an sf-integer or sf-decimal; ParseFloat covers both.
		seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(seconds) || seconds <= 0 || math.IsInf(seconds, 1) {
			return 0, false
		}

		return time.Duration(seconds * float64(time.Second)), true
	}

	return 0, false
}
