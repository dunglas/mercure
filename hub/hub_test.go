package hub

import (
	"os"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAddr = "127.0.0.1:4242"

func TestNewHub(t *testing.T) {
	h := createDummy()

	assert.IsType(t, &Options{}, h.options)
}

func TestNewHubFromEnv(t *testing.T) {
	os.Setenv("PUBLISHER_JWT_KEY", "foo")
	os.Setenv("JWT_KEY", "bar")
	defer os.Unsetenv("PUBLISHER_JWT_KEY")
	defer os.Unsetenv("JWT_KEY")

	h, err := NewHubFromEnv()
	assert.Nil(t, err)
	require.NotNil(t, h)
	h.Stop()
}

func TestNewHubFromEnvError(t *testing.T) {
	h, err := NewHubFromEnv()
	assert.Nil(t, h)
	assert.Error(t, err)
}

func TestNewHubFromEnvErrorFromTransport(t *testing.T) {
	os.Setenv("PUBLISHER_JWT_KEY", "foo")
	os.Setenv("JWT_KEY", "bar")
	os.Setenv("TRANSPORT_URL", "foo://")
	defer os.Unsetenv("PUBLISHER_JWT_KEY")
	defer os.Unsetenv("JWT_KEY")
	defer os.Unsetenv("TRANSPORT_URL")

	h, err := NewHubFromEnv()
	assert.Nil(t, h)
	assert.Error(t, err)
}

func createDummy() *Hub {
	return NewHub(NewLocalTransport(), &Options{PublisherJWTKey: []byte("publisher"), SubscriberJWTKey: []byte("subscriber"), PublisherJWTAlgorithm: hmacSigningMethod, SubscriberJWTAlgorithm: hmacSigningMethod})
}

func createAnonymousDummy() *Hub {
	return createAnonymousDummyWithTransport(NewLocalTransport())
}

func createAnonymousDummyWithTransport(t Transport) *Hub {
	return NewHub(t, &Options{
		PublisherJWTKey:        []byte("publisher"),
		SubscriberJWTKey:       []byte("subscriber"),
		PublisherJWTAlgorithm:  hmacSigningMethod,
		SubscriberJWTAlgorithm: hmacSigningMethod,
		AllowAnonymous:         true,
		Addr:                   testAddr,
		Compress:               false,
	})
}

func createDummyAuthorizedJWT(h *Hub, publisher bool, targets []string) string {
	var key []byte
	token := jwt.New(jwt.SigningMethodHS256)

	if publisher {
		key = h.options.PublisherJWTKey
		token.Claims = &claims{mercureClaim{Publish: targets}, jwt.StandardClaims{}}
	} else {
		key = h.options.SubscriberJWTKey
		token.Claims = &claims{mercureClaim{Subscribe: targets}, jwt.StandardClaims{}}
	}

	tokenString, _ := token.SignedString(key)

	return tokenString
}

func createDummyUnauthorizedJWT() string {
	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, _ := token.SignedString([]byte("unauthorized"))

	return tokenString
}

func createDummyNoneSignedJWT() string {
	token := jwt.New(jwt.SigningMethodNone)
	// The generated token must have more than 41 chars
	token.Claims = jwt.StandardClaims{Subject: "me"}
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	return tokenString
}
