package mercure

// The media type a Server-Sent Events stream is served as.
const eventStreamContentType = "text/event-stream"

// eventStreamEncoder frames updates as Server-Sent Events
// (text/event-stream), the framing every Mercure subscriber has used since
// the protocol's first version.
type eventStreamEncoder struct{}

func (eventStreamEncoder) contentType() []string { return []string{eventStreamContentType} }

// A bare SSE comment. Go currently provides no better way to flush the
// headers, so writing it is what sends them.
func (eventStreamEncoder) preamble() string { return ":\n" }

func (eventStreamEncoder) encode(u *Update) string { return u.String() }

// An SSE comment, to prevent issues with some proxies and old browsers.
func (eventStreamEncoder) heartbeat() string { return ":\n" }
