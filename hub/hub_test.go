package hub

import (
	"os"
	"os/exec"
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

func TestNewHubWithConfig(t *testing.T) {
	v := viper.New()
	v.Set("publisher_jwt_key", "foo")
	v.Set("jwt_key", "bar")

	h, err := NewHub(v)
	assert.Nil(t, err)
	require.NotNil(t, h)
	h.Stop()
}

func TestNewHubValidationError(t *testing.T) {
	h, err := NewHub(viper.New())
	assert.Nil(t, h)
	assert.Error(t, err)
}

func TestNewHubTransportValidationError(t *testing.T) {
	v := viper.New()
	v.Set("publisher_jwt_key", "foo")
	v.Set("jwt_key", "bar")
	v.Set("transport_url", "foo://")

	h, err := NewHub(v)
	assert.Nil(t, h)
	assert.Error(t, err)
}

func TestStartCrash(t *testing.T) {
	if os.Getenv("BE_START_CRASH") == "1" {
		Start()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestStartCrash") //nolint:gosec
	cmd.Env = append(os.Environ(), "BE_START_CRASH=1")
	err := cmd.Run()

	e, ok := err.(*exec.ExitError)
	require.True(t, ok)
	assert.False(t, e.Success())
}

func createDummy() *Hub {
	v := viper.New()
	SetConfigDefaults(v)
	v.SetDefault("heartbeat_interval", time.Duration(0))
	v.SetDefault("publisher_jwt_key", "publisher")
	v.SetDefault("subscriber_jwt_key", "subscriber")

	return NewHubWithTransport(v, NewLocalTransport())
}

func createAnonymousDummy() *Hub {
	return createDummyWithTransportAndConfig(NewLocalTransport(), viper.New())
}

func createDummyWithTransportAndConfig(t Transport, v *viper.Viper) *Hub {
	SetConfigDefaults(v)
	v.SetDefault("heartbeat_interval", time.Duration(0))
	v.SetDefault("publisher_jwt_key", "publisher")
	v.SetDefault("subscriber_jwt_key", "subscriber")
	v.SetDefault("allow_anonymous", true)
	v.SetDefault("addr", testAddr)

	return NewHubWithTransport(v, t)
}

func createDummyAuthorizedJWT(h *Hub, r role, targets []string) string {
	token := jwt.New(jwt.SigningMethodHS256)
	key := h.getJWTKey(r)

	switch r {
	case publisherRole:
		token.Claims = &claims{mercureClaim{Publish: targets}, jwt.StandardClaims{}}

	case subscriberRole:
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
