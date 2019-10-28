package hub

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/dgrijalva/jwt-go"
)

// Claims contains Mercure's JWT claims
type claims struct {
	Mercure mercureClaim `json:"mercure"`
	jwt.StandardClaims
}

type mercureClaim struct {
	Publish   []string `json:"publish"`
	Subscribe []string `json:"subscribe"`
}

type role int

const (
	subscriberRole role = iota
	publisherRole
)

func (h *Hub) getJWTKey(r role) []byte {
	var configKey string
	switch r {
	case subscriberRole:
		configKey = "subscriber_jwt_key"
	case publisherRole:
		configKey = "publisher_jwt_key"
	}

	key := h.config.GetString(configKey)
	if key == "" {
		key = h.config.GetString("jwt_key")
	}
	if key == "" {
		log.Panicf("one of these configuration parameters must be defined: [%s jwt_key]", configKey)
	}

	return []byte(key)
}

func (h *Hub) getJWTAlgorithm(r role) jwt.SigningMethod {
	var configKey string
	switch r {
	case subscriberRole:
		configKey = "subscriber_jwt_algorithm"
	case publisherRole:
		configKey = "publisher_jwt_algorithm"
	}

	keyType := h.config.GetString(configKey)
	if keyType == "" {
		keyType = h.config.GetString("jwt_algorithm")
	}

	sm := jwt.GetSigningMethod(keyType)
	if nil == sm {
		log.Panicf("invalid signing method: %s", keyType)
	}

	return sm
}

// Authorize validates the JWT that may be provided through an "Authorization" HTTP header or a "mercureAuthorization" cookie.
// It returns the claims contained in the token if it exists and is valid, nil if no token is provided (anonymous mode), and an error if the token is not valid.
func authorize(r *http.Request, jwtKey []byte, jwtSigningAlgorithm jwt.SigningMethod, publishAllowedOrigins []string) (*claims, error) {
	authorizationHeaders, headerExists := r.Header["Authorization"]
	if headerExists {
		if len(authorizationHeaders) != 1 || len(authorizationHeaders[0]) < 48 || authorizationHeaders[0][:7] != "Bearer " {
			return nil, errors.New("invalid \"Authorization\" HTTP header")
		}

		return validateJWT(authorizationHeaders[0][7:], jwtKey, jwtSigningAlgorithm)
	}

	cookie, err := r.Cookie("mercureAuthorization")
	if err != nil {
		// Anonymous
		return nil, nil
	}

	// CSRF attacks cannot occurs when using safe methods
	if r.Method != "POST" {
		return validateJWT(cookie.Value, jwtKey, jwtSigningAlgorithm)
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// Try to extract the origin from the Referer, or return an error
		referer := r.Header.Get("Referer")
		if referer == "" {
			return nil, errors.New("an \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
		}

		u, err := url.Parse(referer)
		if err != nil {
			return nil, err
		}

		origin = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}

	for _, allowedOrigin := range publishAllowedOrigins {
		if origin == allowedOrigin {
			return validateJWT(cookie.Value, jwtKey, jwtSigningAlgorithm)
		}
	}

	return nil, fmt.Errorf("the origin \"%s\" is not allowed to post updates", origin)
}

// validateJWT validates that the provided JWT token is a valid Mercure token
func validateJWT(encodedToken string, key []byte, signingAlgorithm jwt.SigningMethod) (*claims, error) {
	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, func(token *jwt.Token) (interface{}, error) {
		switch signingAlgorithm.(type) {
		case *jwt.SigningMethodHMAC:
			return key, nil
		case *jwt.SigningMethodRSA:
			block, _ := pem.Decode(key)

			if block == nil {
				return nil, errors.New("public key error")
			}

			pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)

			if err != nil {
				return nil, err
			}

			pub := pubInterface.(*rsa.PublicKey)

			return pub, nil
		}

		return nil, fmt.Errorf("unexpected signing method: %T", signingAlgorithm)
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid JWT")
}

func authorizedTargets(claims *claims, publisher bool) (all bool, targets map[string]struct{}) {
	if claims == nil {
		return false, map[string]struct{}{}
	}

	var providedTargets []string
	if publisher {
		providedTargets = claims.Mercure.Publish
	} else {
		providedTargets = claims.Mercure.Subscribe
	}

	authorizedTargets := make(map[string]struct{}, len(providedTargets))
	for _, target := range providedTargets {
		if target == "*" {
			return true, nil
		}

		authorizedTargets[target] = struct{}{}
	}

	return false, authorizedTargets
}
