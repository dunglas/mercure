package mercure

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// RFC 6750 error codes returned in the WWW-Authenticate challenge and used to
// pick the HTTP status code.
const (
	bearerErrInvalidRequest    = "invalid_request"
	bearerErrInvalidToken      = "invalid_token"
	bearerErrInsufficientScope = "insufficient_scope"
)

// writeBearerChallenge answers an unauthenticated request (no token presented)
// with a 401 and a bare RFC 6750 Bearer challenge, advertising the protected
// resource metadata so clients can discover the authorization requirements.
func (h *Hub) writeBearerChallenge(w http.ResponseWriter, r *http.Request) {
	h.setWWWAuthenticate(w, r, "")
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

// writeBearerError answers with an RFC 6750 error: a WWW-Authenticate challenge
// carrying the error code and the matching status.
func (h *Hub) writeBearerError(w http.ResponseWriter, r *http.Request, code string, status int) {
	h.setWWWAuthenticate(w, r, code)
	http.Error(w, http.StatusText(status), status)
}

// writeAuthError maps an authorization failure to the RFC 6750 response:
//   - nil error (no token presented) → 401 bare challenge,
//   - malformed request framing (bad header/query parameter) or a failed
//     cookie CSRF check (missing or disallowed Origin/Referer) →
//     400 invalid_request, since the token itself was never inspected,
//   - any token defect, including malformed authorization details →
//     401 invalid_token. Authorization details live inside the token, so
//     their defects are token-validation failures, and a single error code
//     avoids disclosing that the signature verified but the claims were
//     malformed.
//
// insufficient_scope (403) is written directly by the handlers, which know
// whether a valid token merely lacked a grant.
func (h *Hub) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case err == nil:
		h.writeBearerChallenge(w, r)
	case errors.Is(err, ErrInvalidAuthorizationHeader),
		errors.Is(err, ErrInvalidAuthorizationQuery),
		errors.Is(err, ErrNoOrigin),
		errors.Is(err, ErrOriginNotAllowed):
		h.writeBearerError(w, r, bearerErrInvalidRequest, http.StatusBadRequest)
	default:
		h.writeBearerError(w, r, bearerErrInvalidToken, http.StatusUnauthorized)
	}

	// Token rejections are client-caused and, during a protocol migration,
	// actionable (audience mismatch, missing exp, wrong typ, untrusted issuer,
	// malformed authorization details); surface the reason at info so operators
	// do not have to enable debug logging to diagnose a failing upgrade.
	ctx := r.Context()
	if err != nil && h.logger.Enabled(ctx, slog.LevelInfo) {
		h.logger.LogAttrs(ctx, slog.LevelInfo, "Authorization error", slog.Any("error", err))
	}
}

// setWWWAuthenticate writes the RFC 6750 WWW-Authenticate: Bearer header,
// including the RFC 9728 resource_metadata parameter.
func (h *Hub) setWWWAuthenticate(w http.ResponseWriter, r *http.Request, code string) {
	var b strings.Builder
	b.WriteString("Bearer")

	sep := " "
	if code != "" {
		fmt.Fprintf(&b, `%serror=%q`, sep, code)
		sep = ", "
	}

	fmt.Fprintf(&b, `%sresource_metadata=%q`, sep, h.resourceMetadataURL(r))

	w.Header().Set("WWW-Authenticate", b.String())
}

// resourceMetadataURL returns the absolute URL of the hub's RFC 9728 protected
// resource metadata, derived from the public URL when set, otherwise from the
// request.
func (h *Hub) resourceMetadataURL(r *http.Request) string {
	if h.publicURL != "" {
		if base, _, ok := strings.Cut(h.publicURL, defaultHubURL); ok {
			return base + protectedResourceMetadataPath
		}
	}

	scheme := schemeHTTPS
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != schemeHTTPS {
		scheme = "http"
	}

	return scheme + "://" + r.Host + protectedResourceMetadataPath
}
