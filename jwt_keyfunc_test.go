package mercure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateJWTKeyfunc(t *testing.T) {
	f, err := createJWTKeyfunc(([]byte{}), "invalid")
	require.Error(t, err)
	require.Nil(t, f)
}

func TestAuthorizeAuthorizationHeaderEmptyKeyRsa(t *testing.T) {
	keyfunc, err := createJWTKeyfunc([]byte{}, "RS256")
	require.EqualError(t, err, "unable to parse RSA public key: invalid key: Key must be a PEM encoded PKCS1 or PKCS8 key")
	require.Nil(t, keyfunc)
}

func TestAuthorizeAuthorizationHeaderInvalidKeyRsa(t *testing.T) {
	keyfunc, err := createJWTKeyfunc([]byte(privateKeyRsa), "RS256")
	require.EqualError(t, err, "unable to parse RSA public key: asn1: structure error: integer too large")
	require.Nil(t, keyfunc)
}
