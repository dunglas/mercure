package hub

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	testAddr        = "127.0.0.1:4242"
	testMetricsAddr = "127.0.0.1:4243"
)

func TestNewHub(t *testing.T) {
	h := createDummy()

	assert.IsType(t, &viper.Viper{}, h.config)
}

func TestNewHubWithConfig(t *testing.T) {
	h := New([]byte("foo"), WithSubscriberJWTConfig([]byte("bar"), nil))
	require.NotNil(t, h)
	h.Stop()
}

func TestNewHubValidationError(t *testing.T) {
	assert.Panics(t, func() {
		NewHubFromViper(viper.New())
	})
}

func TestNewHubTransportValidationError(t *testing.T) {
	v := viper.New()
	v.Set("publisher_jwt_key", "foo")
	v.Set("jwt_key", "bar")
	v.Set("transport_url", "foo://")

	assert.Panics(t, func() {
		NewHubFromViper(viper.New())
	})
}

func TestStartCrash(t *testing.T) {
	if os.Getenv("BE_START_CRASH") == "1" {
		Start()

		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestStartCrash") //nolint:gosec
	cmd.Env = append(os.Environ(), "BE_START_CRASH=1")
	err := cmd.Run()

	var e *exec.ExitError
	require.True(t, errors.As(err, &e))
	assert.False(t, e.Success())
}

func createDummy(options ...Option) *Hub {
	options = append(
		[]Option{WithSubscriberJWTConfig([]byte("subscriber"), jwt.SigningMethodHS256), WithLogger(zap.NewNop())},
		options...,
	)

	h := New([]byte("publisher"), options...)
	h.config = viper.New()
	h.config.Set("addr", testAddr)
	h.config.Set("metrics_addr", testMetricsAddr)

	return h
}

func createAnonymousDummy(options ...Option) *Hub {
	options = append(
		[]Option{WithAnonymous()},
		options...,
	)

	return createDummy(options...)
}

func createDummyAuthorizedJWT(h *Hub, r role, topics []string) string {
	token := jwt.New(jwt.SigningMethodHS256)

	var key []byte
	switch r {
	case rolePublisher:
		token.Claims = &claims{Mercure: mercureClaim{Publish: topics}, StandardClaims: jwt.StandardClaims{}}
		key = h.publisherJWTConfig.key

	case roleSubscriber:
		var payload struct {
			Foo string `json:"foo"`
		}
		payload.Foo = "bar"
		token.Claims = &claims{
			Mercure: mercureClaim{
				Subscribe: topics,
				Payload:   payload,
			},
			StandardClaims: jwt.StandardClaims{},
		}

		key = h.subscriberJWTConfig.key
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
