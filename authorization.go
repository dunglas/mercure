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

// claims contains Mercure's JWT claims.
type claims struct {
	jwt.RegisteredClaims

	Mercure mercureClaim `json:"mercure"`
	// Optional fallback
	MercureNamespaced *mercureClaim `json:"https://mercure.rocks/"`
}

type mercureClaim struct {
	Publish   []matcherClaim `json:"publish"`
	Subscribe []matcherClaim `json:"subscribe"`
	Payload   any            `json:"payload"`
}

type role int

const (
	defaultCookieName = "mercureAuthorization"
	bearerPrefix      = "Bearer "
	// authorizationParam is the lowercase name shared by the authorization
	// query parameter and the CORS allowed header.
	authorizationParam = "authorization"
)

const (
	roleSubscriber role = iota
	rolePublisher
)

var (
	// ErrInvalidAuthorizationHeader is returned when the Authorization header is invalid.
	ErrInvalidAuthorizationHeader = errors.New(`invalid "Authorization" HTTP header`)
	// ErrInvalidAuthorizationQuery is returned when the authorization query parameter is invalid.
	ErrInvalidAuthorizationQuery = errors.New(`invalid "authorization" Query parameter`)
	// ErrNoOrigin is returned when the cookie authorization mechanism is used and no Origin nor Referer headers are presents.
	ErrNoOrigin = errors.New(`an "Origin" or a "Referer" HTTP header must be present to use the cookie-based authorization mechanism`)
	// ErrOriginNotAllowed is returned when the Origin is not allowed to post updates.
	ErrOriginNotAllowed = errors.New("origin not allowed to post updates")
	// ErrInvalidJWT is returned when the JWT is invalid.
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

		return validateJWT(authorizationHeaders[0][7:], jwtKeyfunc)
	}

	if authorizationQuery, queryExists := r.URL.Query()[authorizationParam]; queryExists {
		if len(authorizationQuery) != 1 || len(authorizationQuery[0]) < 41 {
			return nil, ErrInvalidAuthorizationQuery
		}

		return validateJWT(authorizationQuery[0], jwtKeyfunc)
	}

	cookie, err := r.Cookie(h.cookieName)
	if err != nil {
		// Anonymous
		return nil, nil //nolint:nilerr,nilnil
	}

	// CSRF attacks cannot occur when using safe methods
	if r.Method != http.MethodPost {
		return validateJWT(cookie.Value, jwtKeyfunc)
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
		return validateJWT(cookie.Value, jwtKeyfunc)
	}

	if slices.Contains(h.publishOrigins, origin) {
		return validateJWT(cookie.Value, jwtKeyfunc)
	}

	for _, allowedOrigin := range h.publishWOrigins {
		if allowedOrigin.match(origin) {
			return validateJWT(cookie.Value, jwtKeyfunc)
		}
	}

	return nil, fmt.Errorf("%q: %w", origin, ErrOriginNotAllowed)
}

// ErrTooManyClaimMatchers is returned when mercure.subscribe or
// mercure.publish exceeds maxClaimMatchers.
var ErrTooManyClaimMatchers = errors.New("too many matchers in mercure claim")

// validateJWT validates that the provided JWT token is a valid Mercure token.
func validateJWT(encodedToken string, jwtKeyfunc jwt.Keyfunc) (*claims, error) {
	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, jwtKeyfunc)
	if err != nil {
		return nil, fmt.Errorf("unable to parse JWT: %w", err)
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidJWT
	}

	if c.MercureNamespaced != nil {
		c.Mercure = *c.MercureNamespaced
	}

	if len(c.Mercure.Publish) > maxClaimMatchers || len(c.Mercure.Subscribe) > maxClaimMatchers {
		return nil, ErrTooManyClaimMatchers
	}

	return c, nil
}

func canReceive(s *TopicSelectorStore, topics []string, matchers []matcherClaim) bool {
	for _, mc := range matchers {
		if s.matchMatcher(topics, mc.TopicMatcher) {
			return true
		}
	}

	return false
}

func canDispatch(s *TopicSelectorStore, topics []string, matchers []matcherClaim) bool {
	singleTopic := make([]string, 1)

	for _, topic := range topics {
		singleTopic[0] = topic

		var matched bool

		for _, mc := range matchers {
			if s.matchMatcher(singleTopic, mc.TopicMatcher) {
				matched = true

				break
			}
		}

		if !matched {
			return false
		}
	}

	return true
}

func (h *Hub) httpAuthorizationError(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

	ctx := r.Context()
	if h.logger.Enabled(ctx, slog.LevelDebug) {
		h.logger.LogAttrs(ctx, slog.LevelDebug, "Topic selectors not matched, not provided or authorization error", slog.Any("error", err))
	}
}

// httpInsufficientScopeError is returned when a token validated successfully but
// does not grant the requested action on the requested topic. The protocol
// requires 403 insufficient_scope here, distinct from the 401 used for a
// missing or invalid token, so clients can tell "re-authenticate" apart from
// "this token is not allowed".
func (h *Hub) httpInsufficientScopeError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope"`)
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)

	ctx := r.Context()
	if h.logger.Enabled(ctx, slog.LevelDebug) {
		h.logger.LogAttrs(ctx, slog.LevelDebug, "Token does not grant the requested action on the requested topic", slog.Any("error", err))
	}
}
