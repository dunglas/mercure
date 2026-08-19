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
)

// Notices about the "Last-Event-ID" query parameter, deprecated since version
// 8 of the protocol. Whether the hub still honours it decides which of the two
// gets logged.
const (
	deprecatedLastEventIDNotice  = "Deprecated: the 'Last-Event-ID' query parameter is deprecated since the version 8 of the protocol, use 'last_event_id' instead."
	unsupportedLastEventIDNotice = `Unsupported: the "Last-Event-ID" query parameter is not supported anymore, use "last_event_id" instead or enable backward compatibility with version 7 of the protocol.`
)

// errUnreadableSubscription rejects a subscription in a media type the hub
// cannot read.
var errUnreadableSubscription = errors.New("unsupported subscription media type")

// subscribeRequest describes the resolved values that the hub needs to process
// the subscription request.
type subscribeRequest struct {
	// values holds the topic matcher parameters in the shape parseMatchers
	// consumes.
	values url.Values

	lastEventID string
	// lastEventIDSet records that a cursor was supplied even when empty.
	lastEventIDSet bool
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
	}, nil
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

// getValueAndExistence returns the source string and if it was specified at all.
func getValueAndExistence(source []string) (string, bool) {
	if len(source) == 0 {
		return "", false
	}

	return source[0], true
}
