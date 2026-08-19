package mercure

// carrierContentTypes are the media types a stream of notifications can be
// served as, most preferred first. An Events Query names what it will read in
// Accept; this list decides what there is to choose from, and what a
// subscription expressing no preference gets.
//
//nolint:gochecknoglobals
var carrierContentTypes = []string{eventStreamContentType}

// responseEncoders provides the framing for each media type
//
//nolint:gochecknoglobals
var responseEncoders = map[string]streamEncoder{
	eventStreamContentType: eventStreamEncoder{},
}

// Mercure ordinarily (without Events Query) serves only Event Stream
//
//nolint:gochecknoglobals
var mercureCarrierContentType = []string{eventStreamContentType}

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
