package mercure

import (
	"errors"
	"fmt"
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
	// resolved into the same shape). Unexported, so it is never (un)marshaled.
	authz *mercureAuthz
}

type role int

const (
	// defaultCookieName is the name of the authorization cookie carrying the
	// access token: the spec-recommended "__Secure-" prefixed name, which user
	// agents refuse over insecure transport. Plain-HTTP deployments (local
	// development) must configure a prefix-less name with WithCookieName. The
	// pre-1.0 name "mercureAuthorization" is accepted as a fallback only in
	// deprecated_claim builds running in compatibility mode.
	defaultCookieName = "__Secure-mercure_access_token"
	bearerPrefix      = "Bearer "
	// minCompactJWSLen is the shortest plausible length of a JWS in compact
	// serialization (two dots plus base64url-encoded header, claims and
	// signature). Anything shorter is garbage and is rejected before signature
	// verification.
	minCompactJWSLen = 41
	// authorizationHeader is the lowercase name of the "Authorization" HTTP
	// header, used in the CORS allowed-headers list.
	authorizationHeader = "authorization"
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
	authorizationHeaders, authorizationHeaderExists := r.Header["Authorization"]
	if authorizationHeaderExists {
		// The token must be at least minCompactJWSLen bytes after the prefix.
		// The auth scheme is matched case-insensitively per RFC 9110 §11.1.
		if len(authorizationHeaders) != 1 || len(authorizationHeaders[0]) < len(bearerPrefix)+minCompactJWSLen ||
			!strings.EqualFold(authorizationHeaders[0][:len(bearerPrefix)], bearerPrefix) {
			return nil, ErrInvalidAuthorizationHeader
		}

		return h.validateJWT(authorizationHeaders[0][len(bearerPrefix):], publish)
	}

	// The deprecated "authorization" query parameter is honored only in
	// deprecated_claim builds running in compatibility mode. The RFC 6750
	// "access_token" query parameter is not accepted: RFC 9700 §4.3.2 forbids
	// passing access tokens in the URI query string.
	if token, ok := h.legacyAuthQueryParam(r); ok {
		return h.validateJWT(token, publish)
	}

	cookie, err := h.readCookie(r)
	if err != nil {
		// Anonymous
		return nil, nil //nolint:nilerr,nilnil
	}

	// CSRF attacks cannot occur when using safe methods
	if r.Method != http.MethodPost {
		return h.validateJWT(cookie.Value, publish)
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
		return h.validateJWT(cookie.Value, publish)
	}

	if slices.Contains(h.publishOrigins, origin) {
		return h.validateJWT(cookie.Value, publish)
	}

	for _, allowedOrigin := range h.publishWOrigins {
		if allowedOrigin.match(origin) {
			return h.validateJWT(cookie.Value, publish)
		}
	}

	return nil, fmt.Errorf("%q: %w", origin, ErrOriginNotAllowed)
}

// jwtParserOptions returns the RFC 9068 parser checks enforced in modern mode:
// a required audience matching the hub's resource identifier and a required
// exp. In compatibility mode (deprecated_claim builds with
// WithProtocolVersionCompatibility) these checks are relaxed. The accepted
// algorithms are pinned here (RFC 8725) so the algorithm can never be taken
// from the token header: they come from the selected issuer's Verifier (a
// Static pins its one algorithm, a KeyFunc its allowlist, defaulting to the
// asymmetric algorithms), so algs is only empty in compatibility mode.
func (h *Hub) jwtParserOptions(algs []string) []jwt.ParserOption {
	var opts []jwt.ParserOption

	if len(algs) > 0 {
		opts = append(opts, jwt.WithValidMethods(algs))
	}

	if h.compatClaimsEnabled() {
		return opts
	}

	opts = append(opts, jwt.WithExpirationRequired())

	// Enforce the audience only when a resource identifier is configured.
	// Compatibility mode on a build without the deprecated_claim tag still
	// reaches this path (compatClaimsEnabled is a no-op stub there) with an
	// empty identifier; golang-jwt treats an empty expected audience as a
	// required claim, so enforcing it would reject every otherwise-valid token.
	if h.resourceIdentifier != "" {
		opts = append(opts, jwt.WithAudience(h.resourceIdentifier))
	}

	return opts
}

// selectVerifier picks the issuer-specific verifier for a token, using the
// token's unverified iss claim as a selection hint only. An unverified iss can
// only select among the issuer bindings established by trusted configuration;
// it never introduces a key source. Compatibility mode does not check the iss
// claim, so it falls back to the sole configured issuer.
func (h *Hub) selectVerifier(encodedToken string, publish bool) (roleVerifier, error) {
	var pre claims
	if _, _, err := jwt.NewParser().ParseUnverified(encodedToken, &pre); err != nil {
		return roleVerifier{}, fmt.Errorf("%w: %w", ErrInvalidJWT, err)
	}

	iv, ok := h.issuers[pre.Issuer]
	if !ok {
		if !h.compatClaimsEnabled() || len(h.issuers) != 1 {
			return roleVerifier{}, fmt.Errorf("%w: untrusted issuer %q", ErrInvalidJWT, pre.Issuer)
		}

		for _, v := range h.issuers {
			iv = v
		}
	}

	rv := iv.subscriber
	if publish {
		rv = iv.publisher
	}

	if rv.keyfunc == nil {
		return roleVerifier{}, fmt.Errorf("%w: no verifier configured for this role", ErrInvalidJWT)
	}

	return rv, nil
}

// validateJWT parses and validates an access token, returning its claims with
// the mercure authorization details resolved into c.authz.
func (h *Hub) validateJWT(encodedToken string, publish bool) (*claims, error) {
	rv, err := h.selectVerifier(encodedToken, publish)
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, rv.keyfunc, h.jwtParserOptions(rv.algorithms)...)
	if err != nil {
		// Signature, audience, expiration and algorithm failures are all
		// invalid-token conditions; classify them as such for RFC 6750.
		return nil, fmt.Errorf("%w: %w", ErrInvalidJWT, err)
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidJWT
	}

	// RFC 9068: reject tokens not issued as JWT access tokens, so a token
	// minted for another purpose (e.g. an OpenID Connect ID Token) is not
	// accepted. The media type is matched case-insensitively, including the
	// optional "application/" prefix. Relaxed in compatibility mode.
	if h.requireATJWT() {
		typ, _ := token.Header["typ"].(string)

		const mediaTypePrefix = "application/"
		if len(typ) >= len(mediaTypePrefix) && strings.EqualFold(typ[:len(mediaTypePrefix)], mediaTypePrefix) {
			typ = typ[len(mediaTypePrefix):]
		}

		if !strings.EqualFold(typ, atJWTType) {
			return nil, fmt.Errorf(`%w: the "typ" header must be %q`, ErrInvalidJWT, atJWTType)
		}
	}

	// RFC 9068 §4: the verified issuer must be one of the configured issuers, so
	// a token signed by a key the hub trusts for one issuer cannot be replayed
	// under another. selectVerifier already keyed the verification on the
	// unverified iss; re-check the verified claim as defense in depth. Relaxed
	// in compatibility mode.
	if !h.compatClaimsEnabled() {
		if _, ok := h.issuers[c.Issuer]; !ok {
			return nil, fmt.Errorf("%w: untrusted issuer %q", ErrInvalidJWT, c.Issuer)
		}
	}

	authz, err := validateAuthorizationDetails(h.topicMatcherStore, c.AuthorizationDetails)
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
