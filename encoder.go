package mercure

// streamEncoder frames updates onto a subscription response stream.
//
// A subscription response is a sequence of framed messages: a preamble
// written with the headers, one message per update, and keep-alives in
// between.
type streamEncoder interface {
	// contentType is the response Content-Type field value.
	contentType() []string

	// preamble is written immediately after the headers and before any
	// update. Writing it is also what forces the headers onto the wire.
	preamble() string

	// encode returns the wire form of a single update.
	encode(u *Update) string

	// heartbeat returns the keep-alive payload.
	heartbeat() string
}
