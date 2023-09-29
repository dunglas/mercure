package mercure

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

// claims contains Mercure's JWT claims.
type claims struct {
	Mercure mercureClaim `json:"mercure"`
	// Optional fallback
	MercureNamespaced *mercureClaim `json:"https://mercure.rocks/"`
	jwt.RegisteredClaims
}

type mercureClaim struct {
	Publish   []string    `json:"publish"`
	Subscribe []string    `json:"subscribe"`
	Payload   interface{} `json:"payload"`
}

type role int

const (
	defaultCookieName      = "mercureAuthorization"
	bearerPrefix           = "Bearer "
	roleSubscriber    role = iota
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
	// ErrUnexpectedSigningMethod is returned when the signing JWT method is not supported.
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
	// ErrInvalidJWT is returned when the JWT is invalid.
	ErrInvalidJWT = errors.New("invalid JWT")
	// ErrPublicKey is returned when there is an error with the public key.
	ErrPublicKey = errors.New("public key error")
	// ErrInvalidJWKS is returned when the JWKS config is invalid.
	ErrInvalidJWKS = errors.New("invalid JWKS")
)

// Authorize validates the JWT that may be provided through an "Authorization" HTTP header or an authorization cookie.
// It returns the claims contained in the token if it exists and is valid, nil if no token is provided (anonymous mode), and an error if the token is not valid.
func authorize(r *http.Request, jwtConfig *jwtConfig, jwks *jwksConfig, publishOrigins []string, cookieName string) (*claims, error) {
	authorizationHeaders, headerExists := r.Header["Authorization"]
	if headerExists {
		if len(authorizationHeaders) != 1 || len(authorizationHeaders[0]) < 48 || authorizationHeaders[0][:7] != bearerPrefix {
			return nil, ErrInvalidAuthorizationHeader
		}

		return validateJWT(authorizationHeaders[0][7:], jwtConfig, jwks)
	}

	if authorizationQuery, queryExists := r.URL.Query()["authorization"]; queryExists {
		if len(authorizationQuery) != 1 || len(authorizationQuery[0]) < 41 {
			return nil, ErrInvalidAuthorizationQuery
		}

		return validateJWT(authorizationQuery[0], jwtConfig, jwks)
	}

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		// Anonymous
		return nil, nil //nolint:nilerr,nilnil
	}

	// CSRF attacks cannot occur when using safe methods
	if r.Method != http.MethodPost {
		return validateJWT(cookie.Value, jwtConfig, jwks)
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

	for _, allowedOrigin := range publishOrigins {
		if allowedOrigin == "*" || origin == allowedOrigin {
			return validateJWT(cookie.Value, jwtConfig, jwks)
		}
	}

	return nil, fmt.Errorf("%q: %w", origin, ErrOriginNotAllowed)
}

// validateJWT validates that the provided JWT token is a valid Mercure token.
func validateJWT(encodedToken string, jwtConfig *jwtConfig, jwksConfig *jwksConfig) (*claims, error) {
	if jwksConfig != nil {
		return validateWithJWKS(encodedToken, jwksConfig)
	}

	return parseJWTClaims(encodedToken, jwtKeyfunc(jwtConfig))
}

func validateWithJWKS(encodedToken string, jwksConfig *jwksConfig) (*claims, error) {
	if jwksConfig.url != "" {
		jwks, err := keyfunc.Get(jwksConfig.url, keyfunc.Options{})
		if err != nil {
			return nil, fmt.Errorf("failed to get the JWKS from the given URL: %w", err)
		}

		return parseJWTClaims(encodedToken, jwks.Keyfunc)
	}

	if jwksConfig.json != "" {
		jwksJSON := json.RawMessage(jwksConfig.json)

		jwks, err := keyfunc.NewJSON(jwksJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to  create JWKS from JSON: %w", err)
		}

		return parseJWTClaims(encodedToken, jwks.Keyfunc)
	}

	if len(jwksConfig.key) == 0 && jwksConfig.keyID != "" {
		uniquekeyID := jwksConfig.keyID
		jwks := keyfunc.NewGiven(map[string]keyfunc.GivenKey{
			uniquekeyID: keyfunc.NewGivenHMAC(jwksConfig.key),
		})

		return parseJWTClaims(encodedToken, jwks.Keyfunc)
	}

	return nil, ErrInvalidJWKS
}

func parseJWTClaims(encodedToken string, keyFunc jwt.Keyfunc) (*claims, error) {
	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, keyFunc)
	if err != nil {
		return nil, fmt.Errorf("unable to parse JWT: %w", err)
	}

	if claims, ok := token.Claims.(*claims); ok && token.Valid {
		if claims.MercureNamespaced != nil {
			claims.Mercure = *claims.MercureNamespaced
		}

		return claims, nil
	}

	return nil, ErrInvalidJWT
}

func jwtKeyfunc(jwtConfig *jwtConfig) func(token *jwt.Token) (interface{}, error) {
	return func(token *jwt.Token) (interface{}, error) {
		switch jwtConfig.signingMethod.(type) {
		case *jwt.SigningMethodHMAC:
			return jwtConfig.key, nil
		case *jwt.SigningMethodRSA:
			pub, err := jwt.ParseRSAPublicKeyFromPEM(jwtConfig.key)
			if err != nil {
				return nil, fmt.Errorf("unable to parse RSA public key: %w", err)
			}

			return pub, nil
		}

		return nil, fmt.Errorf("%T: %w", jwtConfig.signingMethod, ErrUnexpectedSigningMethod)
	}
}

func canReceive(s *TopicSelectorStore, topics, topicSelectors []string) bool {
	for _, topic := range topics {
		for _, topicSelector := range topicSelectors {
			if s.match(topic, topicSelector) {
				return true
			}
		}
	}

	return false
}

func canDispatch(s *TopicSelectorStore, topics, topicSelectors []string) bool {
	for _, topic := range topics {
		var matched bool
		for _, topicSelector := range topicSelectors {
			if topicSelector == "*" {
				return true
			}

			if s.match(topic, topicSelector) {
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
	if c := h.logger.Check(zap.DebugLevel, "Topic selectors not matched, not provided or authorization error"); c != nil {
		c.Write(zap.String("remote_addr", r.RemoteAddr), zap.Error(err))
	}
}
