package mercure

import "encoding/base64"

// The media type a Server-Sent Events stream is served as.
const eventStreamContentType = "text/event-stream"

// eventStreamEncoder frames updates as Server-Sent Events
// (text/event-stream), the framing every Mercure subscriber has used since
// the protocol's first version.
type eventStreamEncoder struct{}

func newEventStreamEncoder() streamEncoder { return eventStreamEncoder{} }

func (eventStreamEncoder) contentType() []string { return []string{eventStreamContentType} }

// A bare SSE comment. Go currently provides no better way to flush the
// headers, so writing it is what sends them.
func (eventStreamEncoder) preamble() string { return ":\n" }

// Binary payloads are always base64-encoded, even when the bytes happen to
// be valid UTF-8: an event stream is a text format with no per-event
// metadata slot, so only a rule fixed at publication time lets subscribers
// decode deterministically. The multipart framing delivers them verbatim.
func (eventStreamEncoder) encode(u *Update) string {
	if !u.Binary {
		return u.String()
	}

	e := u.Event
	e.Data = base64.StdEncoding.EncodeToString([]byte(e.Data))

	return e.String()
}

// An SSE comment, to prevent issues with some proxies and old browsers.
func (eventStreamEncoder) heartbeat() string { return ":\n" }

func (eventStreamEncoder) trailer() string { return "" }
