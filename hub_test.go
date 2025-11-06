package mercure

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAddr        = "127.0.0.1:4242"
	testMetricsAddr = "127.0.0.1:4243"
)

func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Verbose() {
		slog.SetDefault(slog.New(slog.DiscardHandler))
	}

	os.Exit(m.Run())
}

func TestNewHub(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	assert.False(t, h.anonymous)
	assert.Equal(t, defaultCookieName, h.cookieName)
	assert.Equal(t, 40*time.Second, h.heartbeat)
	assert.Equal(t, 5*time.Second, h.dispatchTimeout)
	assert.Equal(t, 600*time.Second, h.writeTimeout)
}

func TestNewHubWithConfig(t *testing.T) {
	t.Parallel()

	h, err := NewHub(
		t.Context(),
		WithPublisherJWT([]byte("foo"), jwt.SigningMethodHS256.Name),
		WithSubscriberJWT([]byte("bar"), jwt.SigningMethodHS256.Name),
	)
	require.NotNil(t, h)
	require.NoError(t, err)
}

func TestStop(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t)
	ctx := t.Context()

	synctest.Test(t, func(t *testing.T) {
		go func() {
			s := hub.transport.(*LocalTransport)

			var ready bool

			for !ready {
				s.RLock()
				ready = s.subscribers.Len() == 2
				s.RUnlock()
			}

			_ = hub.transport.Dispatch(ctx, &Update{
				Topics: []string{"https://example.com/foo"},
				Event:  Event{Data: "Hello World"},
			})

			_ = hub.Stop(ctx)
		}()

		for range 2 {
			go func() {
				req := httptest.NewRequest(http.MethodGet, defaultHubURL+"?topic=https://example.com/foo", nil)

				w := newSubscribeRecorder()
				hub.SubscribeHandler(w, req)

				r := w.Result()
				_ = r.Body.Close()
				assert.Equal(t, 200, r.StatusCode)
			}()
		}

		synctest.Wait()
	})
}

func TestWithProtocolVersionCompatibility(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

	op := &opt{}

	o := WithPublisherJWTKeyFunc(func(_ *jwt.Token) (any, error) { return []byte{}, nil })
	require.NoError(t, o(op))
	require.NotNil(t, op.publisherJWTKeyFunc)
}

func TestWithSubscriberJWTKeyFunc(t *testing.T) {
	t.Parallel()

	op := &opt{}

	o := WithSubscriberJWTKeyFunc(func(_ *jwt.Token) (any, error) { return []byte{}, nil })
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
	t.Parallel()

	op := &opt{}

	o := WithUI()
	require.NoError(t, o(op))
	require.True(t, op.ui)
}

func TestOriginsValidator(t *testing.T) {
	t.Parallel()

	op := &opt{}

	validOrigins := [][]string{
		{"*"},
		{"null"},
		{"https://example.com"},
		{"https://example.com:8000"},
		{"https://example.com", "https://example.org"},
		{"https://example.com", "*"},
		{"null", "https://example.com:3000"},
		{"capacitor://"},
		{"capacitor://www.example.com"},
		{"ionic://"},
		{"foobar://"},
		{"https://*.example.com"},
	}

	invalidOrigins := [][]string{
		{"f"},
		{"foo"},
		{"https://example.com", "bar"},
		{"https://example.com/"},
		{"https://user@example.com"},
		{"https://example.com:abc"},
		{"https://example.com", "https://example.org/hello"},
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

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	hub := createAnonymousDummy(t, WithSubscriptions(), WithCORSOrigins([]string{"https://example.com"}), WithDemo())

	form := url.Values{}
	form.Add("id", "id")
	form.Add("topic", "https://example.com/books/1")
	form.Add("data", "Hello!")
	form.Add("private", "on")

	req := httptest.NewRequest(http.MethodPost, defaultHubURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerPrefix+createDummyAuthorizedJWT(rolePublisher, []string{"*"}))

	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	resp := w.Result()

	t.Cleanup(func() {
		assert.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, "default-src 'self' mercure.rocks cdn.jsdelivr.net", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-Xss-Protection"))
	require.NoError(t, resp.Body.Close())

	// Preflight request
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodOptions, defaultHubURL, nil)
	req.Header.Add("Origin", "https://example.com")
	req.Header.Add("Access-Control-Request-Headers", "authorization,cache-control,last-event-id")
	req.Header.Add("Access-Control-Request-Method", http.MethodGet)
	hub.ServeHTTP(w, req)

	resp2 := w.Result()
	require.NotNil(t, resp2)

	assert.Equal(t, "true", resp2.Header.Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "authorization,cache-control,last-event-id", resp2.Header.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "https://example.com", resp2.Header.Get("Access-Control-Allow-Origin"))
	require.NoError(t, resp2.Body.Close())

	// Subscriptions
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, defaultHubURL+subscriptionsPath, nil)
	hub.ServeHTTP(w, req)
	resp3 := w.Result()

	require.NotNil(t, resp3)
	assert.Equal(t, http.StatusUnauthorized, resp3.StatusCode)
	require.NoError(t, resp3.Body.Close())
}

func TestWithPublishDisabled(t *testing.T) {
	t.Parallel()

	h, err := NewHub(t.Context(), WithAnonymous())
	require.NoError(t, err)

	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, defaultHubURL, nil))

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestWithSubscribeDisabled(t *testing.T) {
	t.Parallel()

	h, err := NewHub(t.Context(), WithPublisherJWT([]byte(""), "HS256"))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, defaultHubURL, nil))

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func createDummy(tb testing.TB, options ...Option) *Hub {
	tb.Helper()

	tss, err := NewTopicSelectorStoreCache(0, 0)
	require.NoError(tb, err)

	options = append(
		[]Option{
			WithPublisherJWT([]byte("publisher"), jwt.SigningMethodHS256.Name),
			WithSubscriberJWT([]byte("subscriber"), jwt.SigningMethodHS256.Name),
			WithTopicSelectorStore(tss),
		},
		options...,
	)

	h, err := NewHub(tb.Context(), options...)
	require.NoError(tb, err)

	setDeprecatedOptions(tb, h)

	return h
}

func createAnonymousDummy(tb testing.TB, options ...Option) *Hub {
	tb.Helper()

	options = append(
		[]Option{WithAnonymous()},
		options...,
	)

	return createDummy(tb, options...)
}

func createDummyAuthorizedJWT(r role, topics []string) string {
	return createDummyAuthorizedJWTWithPayload(r, topics, struct {
		Foo string `json:"foo"`
	}{Foo: "bar"})
}

func createDummyAuthorizedJWTWithPayload(r role, topics []string, payload any) string {
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
