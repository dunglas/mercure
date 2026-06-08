package mercure

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// claims contains the validated claims of a Mercure access token.
type claims struct {
	jwt.RegisteredClaims

	deprecatedMercureClaims //nolint:unused // populated only in deprecated_claim builds

	// AuthorizationDetails carries the RFC 9396 authorization_details claim.
	AuthorizationDetails []authorizationDetail `json:"authorization_details,omitempty"`

	// authz holds the validated mercure authorization details (and, under the
	// deprecated_claim tag in compatibility mode, the legacy mercure claim
	// resolved into the same shape).
	authz *mercureAuthz `json:"-"`
}

type role int

const (
	defaultCookieName = "mercureAuthorization"
	bearerPrefix      = "Bearer "
	// authorizationParam is the lowercase name of the legacy authorization
	// query parameter and the CORS allowed header.
	authorizationParam = "authorization"
	// atJWTType is the required JWT access token "typ" header value (RFC 9068).
	atJWTType = "at+jwt"
)

const (
	roleSubscriber role = iota
	rolePublisher
)

var (
	// ErrInvalidAuthorizationHeader is returned when the Authorization header is invalid.
	ErrInvalidAuthorizationHeader = errors.New(`invalid "Authorization" HTTP header`)
	// ErrInvalidAuthorizationQuery is returned when the access token query parameter is invalid.
	ErrInvalidAuthorizationQuery = errors.New(`invalid "access_token" query parameter`)
	// ErrNoOrigin is returned when the cookie authorization mechanism is used and no Origin nor Referer headers are presents.
	ErrNoOrigin = errors.New(`an "Origin" or a "Referer" HTTP header must be present to use the cookie-based authorization mechanism`)
	// ErrOriginNotAllowed is returned when the Origin is not allowed to post updates.
	ErrOriginNotAllowed = errors.New("origin not allowed to post updates")
	// ErrInvalidJWT is returned when the access token is invalid.
	ErrInvalidJWT = errors.New("invalid JWT")
)

// wildcard has been copied from https://github.com/rs/cors/blob/1084d89a16921942356d1c831fbe523426cf836e/utils.go
// Copyright (c) 2014 Olivier Poitrey <rs@dailymotion.com>
// MIT licensed.
type wildcard struct {
	prefix string
	suffix string
}

func (w wildcard) match(s string) bool {
	return len(s) >= len(w.prefix)+len(w.suffix) &&
		strings.HasPrefix(s, w.prefix) &&
		strings.HasSuffix(s, w.suffix)
}

// authorize validates the JWT that may be provided through an "Authorization" HTTP header or an authorization cookie.
// It returns the claims contained in the token if it exists and is valid, nil if no token is provided (anonymous mode), and an error if the token is not valid.
func (h *Hub) authorize(r *http.Request, publish bool) (*claims, error) { //nolint:funlen
	var jwtKeyfunc jwt.Keyfunc
	if publish {
		jwtKeyfunc = h.publisherJWTKeyFunc
	} else {
		jwtKeyfunc = h.subscriberJWTKeyFunc
	}

	authorizationHeaders, authorizationHeaderExists := r.Header["Authorization"]
	if authorizationHeaderExists {
		if len(authorizationHeaders) != 1 || len(authorizationHeaders[0]) < 48 || authorizationHeaders[0][:7] != bearerPrefix {
			return nil, ErrInvalidAuthorizationHeader
		}

		return h.validateJWT(authorizationHeaders[0][7:], jwtKeyfunc)
	}

	if accessTokens, queryExists := r.URL.Query()["access_token"]; queryExists {
		if len(accessTokens) != 1 || len(accessTokens[0]) < 41 {
			return nil, ErrInvalidAuthorizationQuery
		}

		return h.validateJWT(accessTokens[0], jwtKeyfunc)
	}

	// The deprecated "authorization" query parameter is honored only in
	// deprecated_claim builds running in compatibility mode.
	if token, ok := h.legacyAuthQueryParam(r); ok {
		return h.validateJWT(token, jwtKeyfunc)
	}

	cookie, err := r.Cookie(h.cookieName)
	if err != nil {
		// Anonymous
		return nil, nil //nolint:nilerr,nilnil
	}

	// CSRF attacks cannot occur when using safe methods
	if r.Method != http.MethodPost {
		return h.validateJWT(cookie.Value, jwtKeyfunc)
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// Try to extract the origin from the Referer, or return an error
		referer := r.Header.Get("Referer")
		if referer == "" {
			return nil, ErrNoOrigin
		}

		u, err := url.Parse(referer)
		if err != nil {
			return nil, fmt.Errorf("unable to parse referer: %w", err)
		}

		origin = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	if h.publishOriginsAll {
		return h.validateJWT(cookie.Value, jwtKeyfunc)
	}

	if slices.Contains(h.publishOrigins, origin) {
		return h.validateJWT(cookie.Value, jwtKeyfunc)
	}

	for _, allowedOrigin := range h.publishWOrigins {
		if allowedOrigin.match(origin) {
			return h.validateJWT(cookie.Value, jwtKeyfunc)
		}
	}

	return nil, fmt.Errorf("%q: %w", origin, ErrOriginNotAllowed)
}

// jwtParserOptions returns the RFC 9068 parser checks enforced in modern mode:
// a required audience matching the hub's resource identifier and a required
// exp. In compatibility mode (deprecated_claim builds with
// WithProtocolVersionCompatibility) these checks are relaxed.
func (h *Hub) jwtParserOptions() []jwt.ParserOption {
	if h.compatClaimsEnabled() {
		return nil
	}

	return []jwt.ParserOption{
		jwt.WithAudience(h.resourceIdentifier),
		jwt.WithExpirationRequired(),
	}
}

// validateJWT parses and validates an access token, returning its claims with
// the mercure authorization details resolved into c.authz.
func (h *Hub) validateJWT(encodedToken string, jwtKeyfunc jwt.Keyfunc) (*claims, error) {
	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, jwtKeyfunc, h.jwtParserOptions()...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse JWT: %w", err)
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidJWT
	}

	// RFC 9068: reject tokens not issued as JWT access tokens, so a token
	// minted for another purpose (e.g. an OpenID Connect ID Token) is not
	// accepted. Relaxed in compatibility mode.
	if h.requireATJWT() {
		if typ, _ := token.Header["typ"].(string); typ != atJWTType {
			return nil, fmt.Errorf(`%w: the "typ" header must be %q`, ErrInvalidJWT, atJWTType)
		}
	}

	authz, err := validateAuthorizationDetails(h.topicSelectorStore, c.AuthorizationDetails)
	if err != nil {
		return nil, err
	}

	c.authz = authz

	// The legacy mercure claim is honored only when the token carries no
	// authorization_details, and only in deprecated_claim builds running in
	// compatibility mode (the stub is a no-op otherwise).
	if len(c.AuthorizationDetails) == 0 {
		if err := h.resolveLegacyClaims(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (h *Hub) httpAuthorizationError(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

	ctx := r.Context()
	if h.logger.Enabled(ctx, slog.LevelDebug) {
		h.logger.LogAttrs(ctx, slog.LevelDebug, "Topic selectors not matched, not provided or authorization error", slog.Any("error", err))
	}
}
