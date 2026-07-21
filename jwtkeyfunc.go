package mercure

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ErrUnexpectedSigningMethod is returned when the signing JWT method is not supported.
var ErrUnexpectedSigningMethod = errors.New("unexpected signing method")

// Issuer binds a trusted issuer (RFC 9068 §4) to its per-role verification
// material. Configure issuers with WithIssuers.
type Issuer struct {
	// Identifier is the exact value of the token iss claim.
	Identifier string
	// AuthorizationServer advertises Identifier in the hub's RFC 9728 protected
	// resource metadata. Leave false for self-issued tokens (a key shared out
	// of band, no authorization server).
	AuthorizationServer bool
	// Publisher and Subscriber verify tokens for each role. A nil Verifier
	// means the role is not accepted for this issuer.
	Publisher  Verifier
	Subscriber Verifier
}

// Verifier supplies the material to verify an access token for one role of one
// issuer. It is a sealed interface with two implementations, Static and
// KeyFunc.
type Verifier interface {
	// buildKeyfunc returns the verification keyfunc and the pinned JWS
	// algorithm allowlist (RFC 8725). Unexported so the set of implementations
	// stays closed to this package.
	buildKeyfunc() (jwt.Keyfunc, []string, error)
}

// Static verifies tokens with a single embedded key.
type Static struct {
	// Key is the verification key: an HMAC secret or a PEM-encoded public key.
	Key []byte
	// Algorithm is the single accepted JWS algorithm (e.g. "HS256", "RS256").
	Algorithm string
}

func (s Static) buildKeyfunc() (jwt.Keyfunc, []string, error) {
	if s.Algorithm == "" {
		return nil, nil, ErrMissingAlgorithm
	}

	keyfunc, err := createJWTKeyfunc(s.Key, s.Algorithm)
	if err != nil {
		return nil, nil, err
	}

	return keyfunc, []string{s.Algorithm}, nil
}

// KeyFunc verifies tokens with a caller-supplied keyfunc, typically backed by a
// JWK Set.
type KeyFunc struct {
	// Keyfunc supplies the verification key(s) for each token.
	Keyfunc jwt.Keyfunc
	// Algorithms pins the accepted JWS algorithms (RFC 8725): the algorithm is
	// never taken from the token header. Defaults to the asymmetric allowlist
	// when empty, so a public JWK can never be reinterpreted as an HMAC secret.
	Algorithms []string
}

func (k KeyFunc) buildKeyfunc() (jwt.Keyfunc, []string, error) {
	algs := k.Algorithms
	if len(algs) == 0 {
		algs = defaultJWTAlgorithms
	}

	return k.Keyfunc, algs, nil
}

func createJWTKeyfunc(key []byte, alg string) (jwt.Keyfunc, error) {
	signingMethod := jwt.GetSigningMethod(alg)

	var k any

	switch signingMethod.(type) {
	case *jwt.SigningMethodHMAC:
		k = key
	case *jwt.SigningMethodRSA:
		pub, err := jwt.ParseRSAPublicKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse RSA public key: %w", err)
		}

		k = pub
	case *jwt.SigningMethodECDSA:
		pub, err := jwt.ParseECPublicKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse EC public key: %w", err)
		}

		k = pub
	case *jwt.SigningMethodEd25519:
		pub, err := jwt.ParseEdPublicKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse Ed public key: %w", err)
		}

		k = pub
	default:
		return nil, fmt.Errorf("%T: %w", signingMethod, ErrUnexpectedSigningMethod)
	}

	return func(t *jwt.Token) (any, error) {
		if t.Method != signingMethod {
			return nil, fmt.Errorf("%T: %w", t.Method, ErrUnexpectedSigningMethod)
		}

		return k, nil
	}, nil
}
