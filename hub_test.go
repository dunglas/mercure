package mercure

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

	assert.False(t, h.opt.anonymous)
	assert.Equal(t, defaultCookieName, h.opt.cookieName)
	assert.Equal(t, 40*time.Second, h.opt.heartbeat)
	assert.Equal(t, 5*time.Second, h.opt.dispatchTimeout)
	assert.Equal(t, 600*time.Second, h.opt.writeTimeout)
}

func TestNewHubWithConfig(t *testing.T) {
	h, err := NewHub(
		WithPublisherJWT([]byte("foo"), jwt.SigningMethodHS256.Name),
		WithSubscriberJWT([]byte("bar"), jwt.SigningMethodHS256.Name),
	)
	require.NotNil(t, h)
	require.NoError(t, err)
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
	require.ErrorAs(t, err, &e)
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
			req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=http://example.com/foo", nil)

			w := newSubscribeRecorder()
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
	require.NoError(t, o(op))
	assert.Equal(t, 7, op.protocolVersionCompatibility)
	assert.True(t, op.isBackwardCompatiblyEnabledWith(7))
	assert.True(t, op.isBackwardCompatiblyEnabledWith(8))
	assert.False(t, op.isBackwardCompatiblyEnabledWith(6))
}

func TestWithProtocolVersionCompatibilityVersions(t *testing.T) {
	op := &opt{}

	testCases := []struct {
		version int
		ok      bool
	}{
		{5, false},
		{6, false},
		{7, true},
		{8, false},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("version %d", tc.version), func(t *testing.T) {
			o := WithProtocolVersionCompatibility(tc.version)

			if tc.ok {
				require.NoError(t, o(op))
			} else {
				require.Error(t, o(op))
			}
		})
	}
}

func TestWithPublisherJWTKeyFunc(t *testing.T) {
	op := &opt{}

	o := WithPublisherJWTKeyFunc(func(_ *jwt.Token) (interface{}, error) { return []byte{}, nil })
	require.NoError(t, o(op))
	require.NotNil(t, op.publisherJWTKeyFunc)
}

func TestWithSubscriberJWTKeyFunc(t *testing.T) {
	op := &opt{}

	o := WithSubscriberJWTKeyFunc(func(_ *jwt.Token) (interface{}, error) { return []byte{}, nil })
	require.NoError(t, o(op))
	require.NotNil(t, op.subscriberJWTKeyFunc)
}

func TestWithDebug(t *testing.T) {
	op := &opt{}

	o := WithDebug()
	require.NoError(t, o(op))
	require.True(t, op.debug)
}

func TestWithUI(t *testing.T) {
	op := &opt{}

	o := WithUI()
	require.NoError(t, o(op))
	require.True(t, op.ui)
}

func TestOriginsValidator(t *testing.T) {
	op := &opt{}

	validOrigins := [][]string{
		{"*"},
		{"null"},
		{"https://example.com"},
		{"http://example.com:8000"},
		{"https://example.com", "http://example.org"},
		{"https://example.com", "*"},
		{"null", "https://example.com:3000"},
		{"capacitor://"},
		{"capacitor://www.example.com"},
		{"ionic://"},
		{"foobar://"},
	}

	invalidOrigins := [][]string{
		{"f"},
		{"foo"},
		{"https://example.com", "bar"},
		{"https://example.com/"},
		{"https://user@example.com"},
		{"https://example.com:abc"},
		{"https://example.com", "http://example.org/hello"},
		{"https://example.com?query", "*"},
		{"null", "https://example.com:3000#fragment"},
	}

	for _, origins := range validOrigins {
		o := WithPublishOrigins(origins)
		require.NoError(t, o(op), "error while not expected for %#v", origins)

		o = WithCORSOrigins(origins)
		require.NoError(t, o(op), "error while not expected for %#v", origins)
	}

	for _, origins := range invalidOrigins {
		o := WithPublishOrigins(origins)
		require.Error(t, o(op), "no error while expected for %#v", origins)

		o = WithCORSOrigins(origins)
		require.Error(t, o(op), "no error while expected for %#v", origins)
	}
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

func createDummyAuthorizedJWT(r role, topics []string) string {
	return createDummyAuthorizedJWTWithPayload(r, topics, struct {
		Foo string `json:"foo"`
	}{Foo: "bar"})
}

func createDummyAuthorizedJWTWithPayload(r role, topics []string, payload interface{}) string {
	token := jwt.New(jwt.SigningMethodHS256)

	var key []byte
	switch r {
	case rolePublisher:
		token.Claims = &claims{Mercure: mercureClaim{Publish: topics}, RegisteredClaims: jwt.RegisteredClaims{}}
		key = []byte("publisher")

	case roleSubscriber:
		token.Claims = &claims{
			Mercure: mercureClaim{
				Subscribe: topics,
				Payload:   payload,
			},
			RegisteredClaims: jwt.RegisteredClaims{},
		}

		key = []byte("subscriber")
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
