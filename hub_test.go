package mercure

import (
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v4"
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
	h, err := NewHub(
		WithPublisherJWT([]byte("foo"), jwt.SigningMethodHS256.Name),
		WithSubscriberJWT([]byte("bar"), jwt.SigningMethodHS256.Name),
	)
	require.NotNil(t, h)
	require.Nil(t, err)
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

func TestStop(t *testing.T) {
	numberOfSubscribers := 2
	hub := createAnonymousDummy()

	go func() {
		s := hub.transport.(*LocalTransport)
		var ready bool

		for !ready {
			s.RLock()
			ready = s.subscribers.Len() == numberOfSubscribers
			s.RUnlock()
		}

		hub.transport.Dispatch(&Update{
			Topics: []string{"http://example.com/foo"},
			Event:  Event{Data: "Hello World"},
		})

		hub.Stop()
	}()

	var wg sync.WaitGroup
	wg.Add(numberOfSubscribers)
	for i := 0; i < numberOfSubscribers; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", defaultHubURL+"?topic=http://example.com/foo", nil)

			w := httptest.NewRecorder()
			hub.SubscribeHandler(w, req)

			r := w.Result()
			r.Body.Close()
			assert.Equal(t, 200, r.StatusCode)
		}()
	}

	wg.Wait()
}

func TestWithProtocolVersionCompatibility(t *testing.T) {
	op := &opt{}

	assert.False(t, op.isBackwardCompatiblyEnabledWith(7))

	o := WithProtocolVersionCompatibility(7)
	require.Nil(t, o(op))
	assert.Equal(t, 7, op.protocolVersionCompatibility)
	assert.True(t, op.isBackwardCompatiblyEnabledWith(7))
	assert.True(t, op.isBackwardCompatiblyEnabledWith(8))
	assert.False(t, op.isBackwardCompatiblyEnabledWith(6))
}

func TestInvalidWithProtocolVersionCompatibility(t *testing.T) {
	op := &opt{}

	o := WithProtocolVersionCompatibility(6)
	require.NotNil(t, o(op))
}

func createDummy(options ...Option) *Hub {
	tss, _ := NewTopicSelectorStoreLRU(0, 0)
	options = append(
		[]Option{
			WithPublisherJWT([]byte("publisher"), jwt.SigningMethodHS256.Name),
			WithSubscriberJWT([]byte("subscriber"), jwt.SigningMethodHS256.Name),
			WithLogger(zap.NewNop()),
			WithTopicSelectorStore(tss),
		},
		options...,
	)

	h, _ := NewHub(options...)
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
		token.Claims = &claims{Mercure: mercureClaim{Publish: topics}, RegisteredClaims: jwt.RegisteredClaims{}}
		key = h.publisherJWT.key

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
			RegisteredClaims: jwt.RegisteredClaims{},
		}

		key = h.subscriberJWT.key
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
	token.Claims = jwt.RegisteredClaims{Subject: "me"}
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	return tokenString
}
