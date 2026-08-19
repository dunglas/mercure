package mercure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/elnormous/contenttype"
)

// Notices about the "Last-Event-ID" query parameter, deprecated since version
// 8 of the protocol. Whether the hub still honours it decides which of the two
// gets logged.
const (
	deprecatedLastEventIDNotice  = "Deprecated: the 'Last-Event-ID' query parameter is deprecated since the version 8 of the protocol, use 'last_event_id' instead."
	unsupportedLastEventIDNotice = `Unsupported: the "Last-Event-ID" query parameter is not supported anymore, use "last_event_id" instead or enable backward compatibility with version 7 of the protocol.`
)

var (
	// errUnreadableSubscription rejects a subscription in a media type the hub
	// cannot read.
	errUnreadableSubscription = errors.New("unsupported subscription media type")

	// errUnacceptableSubscription rejects a subscription refusing every media
	// type the hub can stream notifications in, leaving nothing to send it.
	errUnacceptableSubscription = errors.New("unacceptable response media type")

	// errNoEvents rejects a subscription expressing no interest in a stream of
	// notifications. That request is the single notification of the
	// specification's section 8, which this hub does not serve.
	errNoEvents = errors.New(`missing "events": a stream of notifications is the only mode this hub serves`)
)

// subscriptionMediaTypes are the media types a subscription can be expressed
// in, most preferred first. A parser file registers its reader in
// subscriptionParsers under the same media type; this list decides which of
// them are offered, and in what order.
//
//nolint:gochecknoglobals
var subscriptionMediaTypes = []string{urlEncodedMediaType}

// subscriptionParsers is the reader each media type is read by, one per parser
// file.
//
//nolint:gochecknoglobals
var subscriptionParsers = map[string]func(body []byte) (url.Values, error){
	urlEncodedMediaType: parseURLEncoded,
}

// The media types a stream of notifications is negotiated from, parsed once
// from the lists the encoders are registered under.
//
//nolint:gochecknoglobals
var (
	parsedCarriers = parseMediaTypes(carrierContentTypes)
	parsedMercureCarrier = parseMediaTypes(mercureCarrierContentType)
)

// subscribeRequest describes the resolved values that the hub needs to process
// the subscription request.
type subscribeRequest struct {
	// values holds the topic matcher parameters in the shape parseMatchers
	// consumes.
	values url.Values

	lastEventID string
	// lastEventIDSet records that a cursor was supplied even when empty.
	lastEventIDSet bool

	// contentType is the media type the notifications stream is served as.
	// The framing is built from it on the response side, a per-connection
	// framing belonging there rather than to a request.
	contentType string
}

// parseError is a subscription request the hub will not serve, carrying the
// status it is answered with.
type parseError struct {
	status int
	// cause is recorded on the request span. It is not disclosed to the
	// client, which is answered with the status text.
	cause error
}

// parseSubscribeRequest returns the subscription parameters that a hub needs
// to serve notifications, or refuses with the appropriate HTTP status.
func (h *Hub) parseSubscribeRequest(ctx context.Context, r *http.Request) (*subscribeRequest, *parseError) {

	// An Events Query carries the subscription in the request body, in one of
	// the media types realizing the subscription data model. Nothing comes
	// from the URL: the request carries a subscription rather than naming one.
	if h.eventsQuery && r.Method == methodQuery {
		body, readErr := readSubscribeBody(r)
		if readErr != nil {
			return nil, readErr
		}

		// A request naming no media type, or naming one that is not a media
		// type at all, is incorrect by definition and answered 400; one
		// naming a media type no parser reads is unsupported and answered
		// 415, below (RFC 10008, Section 2.3).
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			return nil, &parseError{status: http.StatusBadRequest, cause: err}
		}

		values, parseErr := parseSubscriptionBody(mediaType, body)
		if parseErr != nil {
			return nil, parseErr
		}

		// The hub reads and understands the subscription, but serves only a
		// stream of notifications, so one asking for no events is
		// unprocessable.
		if _, asked := values["events"]; !asked {
			return nil, &parseError{status: http.StatusUnprocessableEntity, cause: errNoEvents}
		}

		// Content Negotiation for the response
		contentType := negotiate(r, parsedCarriers)
		if contentType == "" {
			return nil, &parseError{status: http.StatusNotAcceptable, cause: errUnacceptableSubscription}
		}

		lastEventID, lastEventIDSet := getValueAndExistence(r.Header.Values("Last-Event-ID"))

		if lastEventID == "" {
			id, exists := getValueAndExistence(values["last_event_id"])
			lastEventID, lastEventIDSet = id, lastEventIDSet || exists
		}

		return &subscribeRequest{
			values:         values,
			lastEventID:    lastEventID,
			lastEventIDSet: lastEventIDSet,
			contentType:    contentType,
		}, nil
	}

	// For GET and HEAD the subscription parameters come from the URL query.
	values := r.URL.Query()

	// For QUERY the application/x-www-form-urlencoded request body is parsed and
	// merged on top, so a subscriber can pass topics either way.
	if r.Method == methodQuery {
		body, readErr := readSubscribeBody(r)
		if readErr != nil {
			return nil, readErr
		}

		// A request naming no media type is incorrect by definition, and one
		// naming a media type the hub cannot read as a subscription is
		// unsupported (RFC 10008, Section 2.3). Neither is read as a form:
		// a server does not infer a media type from the content it carries.
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			return nil, &parseError{status: http.StatusBadRequest, cause: err}
		}

		if mediaType != urlEncodedMediaType {
			return nil, &parseError{
				status: http.StatusUnsupportedMediaType,
				cause:  fmt.Errorf("%w: %q", errUnreadableSubscription, mediaType),
			}
		}

		bodyValues, err := parseURLEncoded(body)
		if err != nil {
			return nil, &parseError{
				status: http.StatusBadRequest,
				cause:  fmt.Errorf("invalid %s request body: %w", urlEncodedMediaType, err),
			}
		}

		for k, vs := range bodyValues {
			values[k] = append(values[k], vs...)
		}
	}

	// A client refusing text/event-stream leaves nothing for the hub to send.
	if negotiate(r, parsedMercureCarrier) == "" {
		return nil, &parseError{status: http.StatusNotAcceptable, cause: errUnacceptableSubscription}
	}

	// The following block extracts the Last-Event-ID from possible sources that
	// have the following precedence: the header field, the parameter, then the
	// version 7 spelling of the parameter. It also reports whether Last-Event-ID
	// was present at all, even with an empty value: the protocol requires
	// answering with a Mercure-Last-Event-ID response field whenever one was.
	lastEventID, lastEventIDSet := getValueAndExistence(r.Header.Values("Last-Event-ID"))

	if lastEventID == "" {
		id, exists := getValueAndExistence(values["last_event_id"])
		lastEventID, lastEventIDSet = id, lastEventIDSet || exists
	}

	if lastEventID == "" {
		id, exists := getValueAndExistence(values["Last-Event-ID"])
		notice := unsupportedLastEventIDNotice

		if h.isBackwardCompatiblyEnabledWith(7) {
			notice = deprecatedLastEventIDNotice
			lastEventID, lastEventIDSet = id, lastEventIDSet || exists
		}

		if exists && h.logger.Enabled(ctx, slog.LevelInfo) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, notice)
		}
	}

	return &subscribeRequest{
		values:         values,
		lastEventID:    lastEventID,
		lastEventIDSet: lastEventIDSet,
		contentType:    eventStreamContentType,
	}, nil
}

// negotiate picks the media type a subscription is answered in.
//
// A subscription naming none of them is not refusing them, so it gets the
// first; one naming several gets whichever it weighted highest. Only a
// subscription that refuses every one of them is answered with an error,
// there being nothing left to send it.
func negotiate(r *http.Request, available []contenttype.MediaType) string {
	// Accept is a list, so field lines repeating it are one list split in two
	// (RFC 9110, Section 5.3). Only the first is read, so a subscription that
	// split its list is negotiated from a request carrying them joined.
	request := r
	if accept := r.Header.Values("Accept"); len(accept) > 1 {
		request = &http.Request{Header: http.Header{"Accept": {strings.Join(accept, ", ")}}}
	}

	best, _, err := contenttype.GetAcceptableMediaType(request, available)

	switch {
	case errors.Is(err, contenttype.ErrNoAcceptableTypeFound):
		return ""
	case err != nil:
		// An Accept the hub cannot read states no preference.
		return available[0].MIME()
	default:
		return best.MIME()
	}
}

// parseMediaTypes is the form negotiation compares against, parsed from the
// media types a list offers. The lists themselves stay strings: that is what
// a reader adds a media type to, and what the encoders are registered under.
func parseMediaTypes(available []string) []contenttype.MediaType {
	parsed := make([]contenttype.MediaType, 0, len(available))
	for _, a := range available {
		parsed = append(parsed, contenttype.NewMediaType(a))
	}

	return parsed
}

// readSubscribeBody reads the request body. Its size is bounded by
// limitRequestBody, as for the publish endpoint.
func readSubscribeBody(r *http.Request) ([]byte, *parseError) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		status := http.StatusBadRequest

		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			status = http.StatusRequestEntityTooLarge
		}

		return nil, &parseError{status: status, cause: fmt.Errorf("reading QUERY request body: %w", err)}
	}

	return body, nil
}

// parseSubscriptionBody reads a body in the media type it declares, or refuses a
// media type no parser reads (RFC 10008, Section 2.3).
func parseSubscriptionBody(mediaType string, body []byte) (url.Values, *parseError) {
	for _, offered := range subscriptionMediaTypes {
		if offered == mediaType {
			values, err := subscriptionParsers[mediaType](body)
			if err != nil {
				return nil, &parseError{
					status: http.StatusBadRequest,
					cause:  fmt.Errorf("invalid %s request body: %w", mediaType, err),
				}
			}

			return values, nil
		}
	}

	return nil, &parseError{
		status: http.StatusUnsupportedMediaType,
		cause:  fmt.Errorf("%w: %q", errUnreadableSubscription, mediaType),
	}
}

// getValueAndExistence returns the source string and if it was specified at all.
func getValueAndExistence(source []string) (string, bool) {
	if len(source) == 0 {
		return "", false
	}

	return source[0], true
}
