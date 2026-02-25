package mercure

import (
	"embed"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"time"
)

const hubLink = "<" + defaultHubURL + `>; rel="mercure"`

// uiContent is our static web server content.
//
//go:embed public
var uiContent embed.FS

// Demo exposes INSECURE Demo endpoints to test discovery and authorization mechanisms.
// Add a query parameter named "body" to define the content to return in the response's body.
// Add a query parameter named "jwt" set a "mercureAuthorization" cookie containing this token.
// The Content-Type header will automatically be set according to the URL's extension.
func (h *Hub) Demo(w http.ResponseWriter, r *http.Request) {
	// JSON-LD is the preferred format
	_ = mime.AddExtensionType(".jsonld", "application/ld+json")
	url := r.URL.String()
	mimeType := mime.TypeByExtension(filepath.Ext(r.URL.Path))

	query := r.URL.Query()
	body := query.Get("body")
	jwt := query.Get("jwt")

	// Several Link headers are set on purpose to allow testing advanced discovery mechanism
	header := w.Header()

	if h.cookieName == defaultCookieName {
		header["Link"] = append(header["Link"], hubLink, "<"+url+`>; rel="self"`)
	} else {
		header["Link"] = append(header["Link"], hubLink+`; cookie-name="`+h.cookieName+`"`, "<"+url+`>; rel="self"`)
	}

	if mimeType != "" {
		header["Content-Type"] = []string{mimeType}
	}

	// Determine if connection is secure (direct TLS or behind TLS-terminating proxy like ALB).
	// Trusts X-Forwarded-Proto unconditionally. This is safe because:
	//   - Local config: r.TLS is always non-nil (direct TLS), so this branch is not reached.
	//   - ALB config: the server runs inside a VPC; external clients cannot inject this header.
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

	cookie := &http.Cookie{
		Name:     h.cookieName,
		Path:     defaultHubURL,
		Value:    jwt,
		HttpOnly: true,     // Always: JS doesn't need cookie access, prevents XSS token theft
		Secure:   isSecure, // Only send over HTTPS (works with direct TLS and reverse proxies)
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
			h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write demo response", slog.Any("error", err))
		}
	}
}
