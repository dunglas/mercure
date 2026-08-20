package mercure

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
)

// Content type for the Events Query response
const multipartDigestContentType = "multipart/digest"

// Default media type of the update data sent to/from the hub.
const updateDataContentType = "text/plain; charset=utf-8"

// Opens every boundary, as many browsers and HTTP libraries write it.
const boundaryPrefix = "----------------"

// multipartDigestEncoder frames updates as a multipart/digest stream. A part
// defaults to message/rfc822 (RFC 2046 §5.1.5), so a notification is a message:
// the event's fields as header fields, its data as the body.
type multipartDigestEncoder struct {
	boundary string
}

// newMultipartDigestEncoder returns an encoder with a fresh random boundary.
func newMultipartDigestEncoder() streamEncoder {
	var b [8]byte
	rand.Read(b[:])

	return &multipartDigestEncoder{boundary: boundaryPrefix + hex.EncodeToString(b[:])}
}

func (e *multipartDigestEncoder) contentType() []string {
	return []string{multipartDigestContentType + `; boundary="` + e.boundary + `"`}
}

// The delimiter opening the first part. The CRLF that terminates it belongs
// to the part itself, so that every part including the terminal is written the
// same way.
func (e *multipartDigestEncoder) preamble() string { return "--" + e.boundary }

// encode writes the notification as a message/rfc822 body wrapped in a
// multipart framing.
func (e *multipartDigestEncoder) encode(u *Update) string {
	var m strings.Builder

	m.WriteString("Event-ID: ")
	m.WriteString(u.ID)
	m.WriteString("\r\n")

	// Mirrors Event.String(): the optional fields are omitted when unset
	// rather than sent empty.
	if u.Type != "" {
		m.WriteString("Event-Type: ")
		m.WriteString(u.Type)
		m.WriteString("\r\n")
	}

	// Milliseconds, as the SSE field this re-emits.
	if u.Retry != 0 {
		m.WriteString("Retry: ")
		m.WriteString(strconv.FormatUint(u.Retry, 10))
		m.WriteString("\r\n")
	}

	contentType := u.ContentType
	if contentType == "" {
		contentType = updateDataContentType
	}

	m.WriteString("Content-Type: ")
	m.WriteString(contentType)
	m.WriteString("\r\nContent-Length: ")
	m.WriteString(strconv.Itoa(len(u.Data)))
	m.WriteString("\r\n\r\n")
	m.WriteString(u.Data)

	return e.notification("", m.String())
}

// An empty part is sent for the heartbeat. An internet linebreak is the
// smallest rfc822 message.
func (e *multipartDigestEncoder) heartbeat() string { return e.notification("", "\r\n") }

// Promotes the delimiter written by the last part into the close-delimiter.
func (e *multipartDigestEncoder) trailer() string { return "--\r\n" }

// Assembles the notification, closing it with the boundary delimiter.
// Thus, a receiver knows that the message is complete.
func (e *multipartDigestEncoder) notification(headers string, body string) string {
	return "\r\n" + headers + "\r\n" + body + "\r\n--" + e.boundary
}
