package mercure

import (
	"embed"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const hubLink = "<" + defaultHubURL + `>; rel="mercure"`

// debuggerContent is our static web server content.
//
//go:embed public
var debuggerContent embed.FS

// Playground exposes INSECURE endpoints to test discovery and authorization mechanisms.
// Add a query parameter named "body" to define the content to return in the response's body.
// Add a query parameter named "jwt" to set the authorization cookie (see WithCookieName) containing this token.
// The Content-Type header will automatically be set according to the URL's extension.
func (h *Hub) Playground(w http.ResponseWriter, r *http.Request) {
	// JSON-LD is the preferred format
	_ = mime.AddExtensionType(".jsonld", "application/ld+json")
	selfLink := "<" + escapeLinkTarget(r.URL.String()) + `>; rel="self"`
	mimeType := mime.TypeByExtension(filepath.Ext(r.URL.Path))

	query := r.URL.Query()
	body := query.Get("body")
	jwt := query.Get("jwt")

	// Several Link headers are set on purpose to allow testing advanced discovery mechanism
	header := w.Header()

	if h.cookieName == defaultCookieName {
		header["Link"] = append(header["Link"], hubLink, selfLink)
	} else {
		header["Link"] = append(header["Link"], hubLink+`; cookie-name="`+h.cookieName+`"`, selfLink)
	}

	if mimeType != "" {
		header["Content-Type"] = []string{mimeType}
	}

	// Secure / HttpOnly are conditional on TLS so the playground keeps working
	// when served over plain HTTP locally (with a prefix-less cookie name);
	// gosec wants them unconditional. A "__Secure-" prefixed name requires
	// the Secure attribute regardless, or user agents drop the cookie.
	secure := r.TLS != nil || strings.HasPrefix(h.cookieName, "__Secure-")

	cookie := &http.Cookie{ //nolint:gosec
		Name:     h.cookieName,
		Path:     defaultHubURL,
		Value:    jwt,
		Secure:   secure,
		HttpOnly: r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	}
	if jwt == "" {
		// Remove cookie if not provided, to be sure a previous one doesn't exist
		cookie.Expires = time.Unix(0, 0)
	}

	http.SetCookie(w, cookie)

	if _, err := io.WriteString(w, body); err != nil { //nolint:gosec
		ctx := r.Context()

		if h.logger.Enabled(ctx, slog.LevelInfo) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write playground response", slog.Any("error", err))
		}
	}
}

// escapeLinkTarget percent-encodes bytes RFC 3986 forbids so the target cannot break out of <...>.
func escapeLinkTarget(u string) string {
	const upperhex = "0123456789ABCDEF"

	var b strings.Builder

	for i := range len(u) {
		c := u[i]
		if isURIByte(c) {
			b.WriteByte(c)

			continue
		}

		b.WriteByte('%')
		b.WriteByte(upperhex[c>>4])
		b.WriteByte(upperhex[c&0xF])
	}

	return b.String()
}

func isURIByte(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' ||
		strings.IndexByte("-._~:/?#[]@!$&'()*+,;=%", c) >= 0
}
