package mercure

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ErrUnexpectedSigningMethod is returned when the signing JWT method is not supported.
var ErrUnexpectedSigningMethod = errors.New("unexpected signing method")

func createJWTKeyfunc(key []byte, alg string) (jwt.Keyfunc, error) {
	signingMethod := jwt.GetSigningMethod(alg)

	var k interface{}
	switch signingMethod.(type) {
	case *jwt.SigningMethodHMAC:
		k = key
	case *jwt.SigningMethodRSA:
		pub, err := jwt.ParseRSAPublicKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse RSA public key: %w", err)
		}

		k = pub
	default:
		return nil, fmt.Errorf("%T: %w", signingMethod, ErrUnexpectedSigningMethod)
	}

	return func(t *jwt.Token) (interface{}, error) {
		if t.Method != signingMethod {
			return nil, fmt.Errorf("%T: %w", t.Method, ErrUnexpectedSigningMethod)
		}

		return k, nil
	}, nil
}
