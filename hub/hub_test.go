package hub

import (
	"os"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAddr = "127.0.0.1:4242"

func TestNewHub(t *testing.T) {
	h := createDummy()

	assert.IsType(t, &viper.Viper{}, h.config)
}

func TestNewHubFromEnv(t *testing.T) {
	os.Setenv("PUBLISHER_JWT_KEY", "foo")
	os.Setenv("JWT_KEY", "bar")
	defer os.Unsetenv("PUBLISHER_JWT_KEY")
	defer os.Unsetenv("JWT_KEY")

	h, err := NewHubFromConfig()
	assert.Nil(t, err)
	require.NotNil(t, h)
	h.Stop()
}

func TestNewHubFromEnvError(t *testing.T) {
	h, err := NewHubFromConfig()
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

	h, err := NewHubFromConfig()
	assert.Nil(t, h)
	assert.Error(t, err)
}

func createDummy() *Hub {
	v := viper.New()
	setConfigDefaults(v)
	v.SetDefault("heartbeat_interval", time.Duration(0))
	v.SetDefault("publisher_jwt_key", "publisher")
	v.SetDefault("subscriber_jwt_key", "subscriber")

	return NewHub(NewLocalTransport(), v)
}

func createAnonymousDummy() *Hub {
	return createDummyWithTransportAndConfig(NewLocalTransport(), viper.New())
}

func createDummyWithTransportAndConfig(t Transport, v *viper.Viper) *Hub {
	setConfigDefaults(v)
	v.SetDefault("heartbeat_interval", time.Duration(0))
	v.SetDefault("publisher_jwt_key", "publisher")
	v.SetDefault("subscriber_jwt_key", "subscriber")
	v.SetDefault("allow_anonymous", true)
	v.SetDefault("addr", testAddr)

	return NewHub(t, v)
}

func createDummyAuthorizedJWT(h *Hub, r role, targets []string) string {
	token := jwt.New(jwt.SigningMethodHS256)
	key := h.getJWTKey(r)

	switch r {
	case publisherRole:
		token.Claims = &claims{mercureClaim{Publish: targets}, jwt.StandardClaims{}}
		break

	case subscriberRole:
		token.Claims = &claims{mercureClaim{Subscribe: targets}, jwt.StandardClaims{}}
		break
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
