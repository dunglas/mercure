package mercure

import (
	jsonv2 "encoding/json/v2"
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
	// legacy marks a 0.x token accepted under compatibility mode.
	legacy bool
}

// UnmarshalJSON decodes the claim set with encoding/json/v2, like the authorization details.
func (c *claims) UnmarshalJSON(data []byte) error {
	type plainClaims claims

	return jsonv2.Unmarshal(data, (*plainClaims)(c)) //nolint:wrapcheck
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
	// maxCompactJWSLen bounds the unverified input decoded before the signature check.
	maxCompactJWSLen = 64 << 10
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
	// The expected audience is the hub's per-request resource identifier, so a
	// token minted for the public URL the client contacted is accepted while one
	// minted for a different host is rejected (RFC 9068).
	expectedAudience, _ := h.requestIdentity(r)

	authorizationHeaders, authorizationHeaderExists := r.Header["Authorization"]
	if authorizationHeaderExists {
		// The token must be at least minCompactJWSLen bytes after the prefix.
		// The auth scheme is matched case-insensitively per RFC 9110 §11.1.
		if len(authorizationHeaders) != 1 || len(authorizationHeaders[0]) < len(bearerPrefix)+minCompactJWSLen ||
			!strings.EqualFold(authorizationHeaders[0][:len(bearerPrefix)], bearerPrefix) {
			return nil, ErrInvalidAuthorizationHeader
		}

		return h.validateJWT(authorizationHeaders[0][len(bearerPrefix):], publish, expectedAudience)
	}

	// The deprecated "authorization" query parameter is honored only in
	// deprecated_claim builds running in compatibility mode. The RFC 6750
	// "access_token" query parameter is not accepted: RFC 9700 §4.3.2 forbids
	// passing access tokens in the URI query string.
	if token, ok := h.legacyAuthQueryParam(r); ok {
		return h.validateLegacyCarriedJWT(token, publish, expectedAudience)
	}

	cookie, legacyCookie, err := h.readCookie(r)
	if err != nil {
		// Anonymous
		return nil, nil //nolint:nilerr,nilnil
	}

	validate := h.validateJWT
	if legacyCookie {
		validate = h.validateLegacyCarriedJWT
	}

	// CSRF attacks cannot occur when using safe methods
	if r.Method != http.MethodPost {
		return validate(cookie.Value, publish, expectedAudience)
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
		return validate(cookie.Value, publish, expectedAudience)
	}

	if slices.Contains(h.publishOrigins, origin) {
		return validate(cookie.Value, publish, expectedAudience)
	}

	for _, allowedOrigin := range h.publishWOrigins {
		if allowedOrigin.match(origin) {
			return validate(cookie.Value, publish, expectedAudience)
		}
	}

	return nil, fmt.Errorf("%q: %w", origin, ErrOriginNotAllowed)
}

// jwtParserOptions returns the RFC 9068 parser checks: a required audience
// matching the hub's per-request resource identifier (expectedAudience, never
// empty: validateJWT refuses that) and a required exp. Legacy tokens (see
// validateJWT) skip them. The accepted algorithms are pinned here (RFC 8725)
// so the algorithm can never be taken from the token header: they come from
// the selected issuer's Verifier (a Static pins its one algorithm, a KeyFunc
// its allowlist, defaulting to the asymmetric algorithms).
func jwtParserOptions(algs []string, expectedAudience string, legacy bool) []jwt.ParserOption {
	var opts []jwt.ParserOption

	if len(algs) > 0 {
		opts = append(opts, jwt.WithValidMethods(algs))
	}

	if legacy {
		return opts
	}

	return append(opts, jwt.WithExpirationRequired(), jwt.WithAudience(expectedAudience))
}

// soleIssuerFallback reports whether a token whose iss names no configured
// issuer may be verified by the sole configured one. Legacy tokens may, as 0.x
// did not check iss; other tokens only when that issuer is the implicit one of
// the deprecated top-level directives, which has no identifier to match.
func (h *Hub) soleIssuerFallback(legacy bool) bool {
	if !h.compatClaimsEnabled() || len(h.issuers) != 1 {
		return false
	}

	_, implicit := h.issuers[""]

	return legacy || implicit
}

// selectVerifier picks the issuer-specific verifier for a token, using the
// token's unverified iss claim as a selection hint only. An unverified iss can
// only select among the issuer bindings established by trusted configuration;
// it never introduces a key source.
func (h *Hub) selectVerifier(issuer string, publish, legacy bool) (roleVerifier, error) {
	iv, ok := h.issuers[issuer]
	if !ok {
		if !h.soleIssuerFallback(legacy) {
			return roleVerifier{}, fmt.Errorf("%w: untrusted issuer %q", ErrInvalidJWT, issuer)
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

// checkAccessTokenProfile applies the RFC 9068 checks the parser does not. The
// token must be issued as a JWT access token, so one minted for another
// purpose (e.g. an OpenID Connect ID Token) is not accepted; the media type is
// matched case-insensitively, including the optional "application/" prefix. It
// must carry iss (§2.2), which only the implicit issuer could otherwise match.
func checkAccessTokenProfile(token *jwt.Token, c *claims) error {
	typ, _ := token.Header["typ"].(string)

	const mediaTypePrefix = "application/"
	if len(typ) >= len(mediaTypePrefix) && strings.EqualFold(typ[:len(mediaTypePrefix)], mediaTypePrefix) {
		typ = typ[len(mediaTypePrefix):]
	}

	if !strings.EqualFold(typ, atJWTType) {
		return fmt.Errorf(`%w: the "typ" header must be %q`, ErrInvalidJWT, atJWTType)
	}

	if c.Issuer == "" {
		return fmt.Errorf("%w: the token has no issuer", ErrInvalidJWT)
	}

	return nil
}

// validateJWT parses and validates an access token, returning its claims with
// the mercure authorization details resolved into c.authz.
func (h *Hub) validateJWT(encodedToken string, publish bool, expectedAudience string) (*claims, error) {
	if len(encodedToken) > maxCompactJWSLen {
		return nil, fmt.Errorf("%w: the token exceeds %d bytes", ErrInvalidJWT, maxCompactJWSLen)
	}

	var pre claims
	if _, _, err := jwt.NewParser().ParseUnverified(encodedToken, &pre); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJWT, err)
	}

	// Compatibility mode relaxes RFC 9068 only for tokens in the 0.x format, so
	// a token minted for 1.x or for another purpose gets every check.
	legacy := h.compatClaimsEnabled() && len(pre.AuthorizationDetails) == 0 && pre.hasLegacyClaim()

	// Fail closed: with no identity to bind the token to, parsing without
	// jwt.WithAudience accepts one audienced anywhere, or carrying no aud at all.
	if expectedAudience == "" && !legacy {
		return nil, fmt.Errorf("%w: the hub has no resource identifier to check the audience against", ErrInvalidJWT)
	}

	rv, err := h.selectVerifier(pre.Issuer, publish, legacy)
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, rv.keyfunc, jwtParserOptions(rv.algorithms, expectedAudience, legacy)...)
	if err != nil {
		// Signature, audience, expiration and algorithm failures are all
		// invalid-token conditions; classify them as such for RFC 6750.
		return nil, fmt.Errorf("%w: %w", ErrInvalidJWT, err)
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidJWT
	}

	if !legacy {
		if err := checkAccessTokenProfile(token, c); err != nil {
			return nil, err
		}
	}

	// RFC 9068 §4: the verified issuer must be one of the configured issuers.
	// What actually prevents a token signed for one trusted issuer from being
	// accepted under another is selectVerifier, which looks the issuer up before
	// choosing a keyfunc and rejects an unknown one; both parses decode the same
	// payload, so this lookup cannot fail today. It is kept as a guard on that
	// invariant: should selectVerifier ever widen which issuer it falls back to,
	// this is what still refuses the token. Skipped where that fallback is deliberate.
	if !h.soleIssuerFallback(legacy) {
		if _, ok := h.issuers[c.Issuer]; !ok {
			return nil, fmt.Errorf("%w: untrusted issuer %q", ErrInvalidJWT, c.Issuer)
		}
	}

	authz, err := validateAuthorizationDetails(h.topicMatcherStore, c.AuthorizationDetails)
	if err != nil {
		return nil, err
	}

	c.authz = authz
	c.legacy = legacy

	if !legacy {
		dropLegacyClaims(c)
	} else if err := h.resolveLegacyClaims(c); err != nil {
		return nil, err
	}

	return c, nil
}

// validateLegacyCarriedJWT validates a token read from a carrier only 0.x
// clients use, which must not leak a 1.x token (RFC 9700 §4.3.2).
func (h *Hub) validateLegacyCarriedJWT(encodedToken string, publish bool, expectedAudience string) (*claims, error) {
	c, err := h.validateJWT(encodedToken, publish, expectedAudience)
	if err != nil {
		return nil, err
	}

	if !c.legacy {
		return nil, fmt.Errorf("%w: only 0.x tokens are accepted in the authorization query parameter or the legacy cookie", ErrInvalidJWT)
	}

	return c, nil
}
