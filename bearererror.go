package mercure

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
func (h *Hub) writeBearerChallenge(w http.ResponseWriter) {
	// An empty error code makes setWWWAuthenticate omit the error= parameter, so
	// this is the bare challenge for an unauthenticated request.
	h.writeBearerError(w, "", http.StatusUnauthorized)
}

// writeBearerError answers with an RFC 6750 error: a WWW-Authenticate challenge
// carrying the error code and the matching status.
func (h *Hub) writeBearerError(w http.ResponseWriter, code string, status int) {
	h.setWWWAuthenticate(w, code)
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
		h.writeBearerChallenge(w)
	case errors.Is(err, ErrInvalidAuthorizationHeader),
		errors.Is(err, ErrNoOrigin),
		errors.Is(err, ErrOriginNotAllowed):
		h.writeBearerError(w, bearerErrInvalidRequest, http.StatusBadRequest)
	default:
		h.writeBearerError(w, bearerErrInvalidToken, http.StatusUnauthorized)
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
// including the RFC 9728 resource_metadata parameter when the hub can build it.
func (h *Hub) setWWWAuthenticate(w http.ResponseWriter, code string) {
	var b strings.Builder
	b.WriteString("Bearer")

	sep := " "
	if code != "" {
		fmt.Fprintf(&b, `%serror=%q`, sep, code)
		sep = ", "
	}

	if h.resourceMetadataURL != "" {
		fmt.Fprintf(&b, `%sresource_metadata=%q`, sep, h.resourceMetadataURL)
	}

	w.Header().Set("WWW-Authenticate", b.String())
}

// buildResourceMetadataURL returns the absolute URL of the hub's RFC 9728
// protected resource metadata, or "" when the hub has no configured origin to
// build it from. It is computed once at configuration time (see
// opt.configureIdentifiers) and cached, since the value never varies per
// request.
//
// RFC 9728 requires an absolute value (a relative reference would be ambiguous
// to clients), and this URL is echoed into the WWW-Authenticate challenge, so
// it is derived only from the configured identity — the public URL, then the
// resource identifier — never from request-supplied Host or X-Forwarded-Proto
// headers, which a client could forge to point discovery at an attacker-chosen
// origin. A token-validating hub in modern mode always has an identifier; the
// empty return is only reached in compatibility mode without one, where the
// parameter is omitted rather than fabricated from the request.
func buildResourceMetadataURL(publicURL, resourceIdentifier string) string {
	for _, identity := range []string{publicURL, resourceIdentifier} {
		if identity == "" {
			continue
		}

		if u, err := url.Parse(identity); err == nil && u.IsAbs() && u.Host != "" {
			return u.Scheme + "://" + u.Host + protectedResourceMetadataPath
		}
	}

	return ""
}
