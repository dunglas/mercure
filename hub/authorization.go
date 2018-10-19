package hub

import (
	"errors"
	"fmt"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
)

// Claims contains Mercure's JWT claims
type claims struct {
	Mercure struct {
		Publish   []string `json:"publish"`
		Subscribe []string `json:"subscribe"`
	} `json:"mercure"`
	jwt.StandardClaims
}

// Authorize validates the JWT that may be provided through an "Authorization" HTTP header or a "mercureAuthorization" cookie.
// It returns the claims contained in the token if it exists and is valid, nil if no token is provided (anonymous mode), and an error if the token is not valid.
func authorize(r *http.Request, jwtKey []byte) (*claims, error) {
	authorizationHeaders, headerExists := r.Header["Authorization"]
	if headerExists {
		if len(authorizationHeaders) != 1 || len(authorizationHeaders[0]) < 48 || authorizationHeaders[0][:7] != "Bearer " {
			return nil, errors.New("Invalid \"Authorization\" HTTP header")
		}

		return validateJWT(authorizationHeaders[0][7:], jwtKey)
	}

	cookie, err := r.Cookie("mercureAuthorization")
	if err == nil {
		// TODO: validate origin
		return validateJWT(cookie.Value, jwtKey)
	}

	// Anonymous
	return nil, nil
}

// validateJWT validates that the provided JWT token is a valid Mercure token
func validateJWT(encodedToken string, key []byte) (*claims, error) {
	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return key, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("Invalid JWT")
}

func authorizedTargets(claims *claims) (all bool, targets map[string]struct{}) {
	authorizedTargets := make(map[string]struct{}, len(claims.Mercure.Publish))
	for _, target := range claims.Mercure.Publish {
		if target == "*" {
			return true, nil
		}

		authorizedTargets[target] = struct{}{}
	}

	return false, authorizedTargets
}
