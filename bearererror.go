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
func (h *Hub) writeBearerChallenge(w http.ResponseWriter, r *http.Request) {
	// An empty error code makes setWWWAuthenticate omit the error= parameter, so
	// this is the bare challenge for an unauthenticated request.
	h.writeBearerError(w, r, "", http.StatusUnauthorized)
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
// including the RFC 9728 resource_metadata parameter when the hub can build it.
func (h *Hub) setWWWAuthenticate(w http.ResponseWriter, r *http.Request, code string) {
	var b strings.Builder
	b.WriteString("Bearer")

	sep := " "
	if code != "" {
		fmt.Fprintf(&b, `%serror=%q`, sep, code)
		sep = ", "
	}

	if _, metadataURL := h.requestIdentity(r); metadataURL != "" {
		fmt.Fprintf(&b, `%sresource_metadata=%q`, sep, metadataURL)
	}

	w.Header().Set("WWW-Authenticate", b.String())
}

// requestIdentity returns the hub's OAuth 2.0 resource identifier (RFC 9068
// `aud`) and its RFC 9728 protected resource metadata URL for r. A statically
// configured resource identifier wins and is returned with its precomputed
// metadata URL. Otherwise the identity is derived from the request origin
// resolved by the embedding server (see NewRequestOriginContext), so a hub
// reachable through several public URLs presents each caller the identity of
// the host it contacted without any per-host configuration.
//
// The derived origin is trusted only because the embedding server validated it
// (the Caddy module reads Caddy's request placeholders, gated by the site's
// host matching and the optional public_urls allowlist) — a raw Host or
// X-Forwarded-Proto header could otherwise point discovery at an attacker
// origin. Returns empty strings when no identifier is configured and no origin
// is available (only reached in compatibility mode without an identifier).
func (h *Hub) requestIdentity(r *http.Request) (identifier, metadataURL string) {
	if h.resourceIdentifier != "" {
		return h.resourceIdentifier, h.resourceMetadataURL
	}

	scheme, host := h.requestOrigin(r)
	if host == "" {
		return "", ""
	}

	origin := scheme + "://" + host

	return origin + defaultHubURL, origin + protectedResourceMetadataPath
}

// buildResourceMetadataURL returns the absolute URL of the hub's RFC 9728
// protected resource metadata for a statically configured resource identifier,
// or "" when the identifier is not a usable absolute URL. RFC 9728 requires an
// absolute value (a relative reference would be ambiguous to clients). When no
// identifier is configured the hub derives this URL per request instead (see
// requestIdentity), so this is computed once at configuration time only for the
// static override.
func buildResourceMetadataURL(resourceIdentifier string) string {
	if u, err := url.Parse(resourceIdentifier); err == nil && u.IsAbs() && u.Host != "" {
		return u.Scheme + "://" + u.Host + protectedResourceMetadataPath
	}

	return ""
}
