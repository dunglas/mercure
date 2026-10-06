package mercure

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionLimits(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		limits SubscriptionLimits
		reason string
	}{
		{"total", SubscriptionLimits{Total: 2}, "total"},
		{"token", SubscriptionLimits{Total: 10, PerToken: 2}, "token"},
		{"client", SubscriptionLimits{Total: 10, PerClient: 2}, "client"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			metrics := NewPrometheusMetrics(nil)
			h := createDummy(t, WithSubscriptionLimits(tc.limits), WithMetrics(metrics))
			token := createDummyAuthorizedJWT(roleSubscriber, []string{"*"})
			register := func() (*LocalSubscriber, *httptest.ResponseRecorder) {
				r := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=foo", nil)
				r.Header.Set("Authorization", bearerPrefix+token)
				r.RemoteAddr = "192.0.2.1:1234"
				w := httptest.NewRecorder()
				s, _ := h.registerSubscriber(t.Context(), w, r)

				return s, w
			}

			first, _ := register()
			require.NotNil(t, first)
			t.Cleanup(func() { h.shutdown(t.Context(), first) })

			second, _ := register()
			require.NotNil(t, second)

			rejected, w := register()
			assert.Nil(t, rejected)
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			retryAfter, err := strconv.Atoi(w.Header().Get("Retry-After"))
			require.NoError(t, err)
			assert.GreaterOrEqual(t, retryAfter, 5)
			assert.LessOrEqual(t, retryAfter, 10)
			assertGaugeValue(t, 2, metrics.subscribers)
			assertCounterValue(t, 1, metrics.subscriptionsRejected.WithLabelValues(tc.reason))

			h.shutdown(t.Context(), second)

			replacement, w := register()
			require.NotNil(t, replacement)
			assert.Equal(t, http.StatusOK, w.Code)
			h.shutdown(t.Context(), replacement)
		})
	}
}

func TestSubscriptionCapacityReleasedAfterFailures(t *testing.T) {
	t.Parallel()

	h := createAnonymousDummy(t, WithSubscriptionLimits(SubscriptionLimits{Total: 1}))

	for range 2 {
		r := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match_unknown=foo", nil)
		w := httptest.NewRecorder()
		s, _ := h.registerSubscriber(t.Context(), w, r)
		assert.Nil(t, s)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Zero(t, h.subscriptionLimiter.total)
	}

	transport := &refusingTransport{}
	h = createAnonymousDummy(t, WithTransport(transport), WithSubscriptionLimits(SubscriptionLimits{Total: 1, PerClient: 1}))

	for range 2 {
		r := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=foo", nil)
		w := httptest.NewRecorder()
		s, _ := h.registerSubscriber(t.Context(), w, r)
		assert.Nil(t, s)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Zero(t, h.subscriptionLimiter.total)
		assert.Empty(t, h.subscriptionLimiter.clients)
	}
}

func TestSubscriptionAdmissionIsAtomic(t *testing.T) {
	t.Parallel()

	h := createAnonymousDummy(t, WithSubscriptionLimits(SubscriptionLimits{Total: 3}))
	permits := make(chan *subscriptionPermit, 32)

	var wg sync.WaitGroup

	for range cap(permits) {
		wg.Go(func() {
			r := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)

			p, _ := h.admitSubscription(r, nil)
			if p != nil {
				permits <- p
			}
		})
	}

	wg.Wait()
	close(permits)
	assert.Len(t, permits, 3)

	for p := range permits {
		p.release()
		p.release()
	}

	assert.Zero(t, h.subscriptionLimiter.total)
}

func TestSubscriptionClientIdentityIgnoresForwardedHeaders(t *testing.T) {
	t.Parallel()

	h := createAnonymousDummy(t, WithSubscriptionLimits(SubscriptionLimits{PerClient: 1}))
	r := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.RemoteAddr = "192.0.2.1:1234"
	p, _ := h.admitSubscription(r, nil)

	require.NotNil(t, p)
	defer p.release()

	r.RemoteAddr = "[::ffff:192.0.2.1]:5678"
	r.Header.Set("X-Forwarded-For", "192.0.2.2")
	second, reason := h.admitSubscription(r, nil)
	assert.Nil(t, second)
	assert.Equal(t, "client", reason)
}

func TestSubscriptionClientIdentityGroupsIPv6Prefix(t *testing.T) {
	t.Parallel()

	h := createAnonymousDummy(t, WithSubscriptionLimits(SubscriptionLimits{PerClient: 1}))
	r := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.RemoteAddr = "[2001:db8:1:2::1]:1234"
	p, _ := h.admitSubscription(r, nil)

	require.NotNil(t, p)
	defer p.release()

	r.RemoteAddr = "[2001:db8:1:2:ffff::2]:5678"
	second, reason := h.admitSubscription(r, nil)
	assert.Nil(t, second)
	assert.Equal(t, "client", reason)

	r.RemoteAddr = "[2001:db8:1:3::1]:1234"
	other, _ := h.admitSubscription(r, nil)
	require.NotNil(t, other)
	other.release()
}

func TestSubscriptionClientIPFunc(t *testing.T) {
	t.Parallel()

	h := createAnonymousDummy(t, WithClientIPFunc(func(r *http.Request) string { return r.Header.Get("X-Test-Client") }))

	for _, tc := range []struct{ header, remoteAddr, expected string }{
		{"192.0.2.1", "198.51.100.1:1234", "192.0.2.1"},
		{"::ffff:192.0.2.1", "198.51.100.1:1234", "192.0.2.1"},
		{"2001:db8:1:2::1", "198.51.100.1:1234", "2001:db8:1:2::/64"},
		{"", "198.51.100.1:1234", "198.51.100.1"},
	} {
		r := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Set("X-Test-Client", tc.header)
		r.RemoteAddr = tc.remoteAddr

		assert.Equal(t, tc.expected, h.subscriptionClient(r))
	}
}

func TestSubscriptionLimitsDisabledByDefault(t *testing.T) {
	t.Parallel()

	h := createAnonymousDummy(t)
	assert.Equal(t, SubscriptionLimits{}, h.subscriptionLimits)

	r := httptest.NewRequest(http.MethodGet, defaultHubURL, nil)

	permits := make([]*subscriptionPermit, 3)
	for i := range permits {
		permits[i], _ = h.admitSubscription(r, nil)
		require.NotNil(t, permits[i])
	}

	for _, p := range permits {
		p.release()
	}

	assert.Zero(t, h.subscriptionLimiter.total)
}

func TestSubscriptionLimitsRejectNegativeValues(t *testing.T) {
	t.Parallel()

	for _, limits := range []SubscriptionLimits{{Total: -1}, {PerToken: -1}, {PerClient: -1}} {
		_, err := NewHub(t.Context(), WithSubscriptionLimits(limits))
		require.ErrorIs(t, err, ErrInvalidSubscriptionLimits)
	}
}

func TestSubscriptionTokenLimitSharesCookieAndHeaderCapacity(t *testing.T) {
	t.Parallel()

	h := createDummy(t, WithSubscriptionLimits(SubscriptionLimits{Total: 3, PerToken: 1}))
	token := createDummyAuthorizedJWT(roleSubscriber, []string{"*"})
	firstRequest := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=foo", nil)
	firstRequest.Header.Set("Authorization", bearerPrefix+token)

	first, _ := h.registerSubscriber(t.Context(), httptest.NewRecorder(), firstRequest)
	require.NotNil(t, first)
	t.Cleanup(func() { h.shutdown(t.Context(), first) })

	cookieRequest := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=foo", nil)
	cookieRequest.AddCookie(&http.Cookie{Name: defaultCookieName, Value: token})
	cookieRequest.RemoteAddr = "192.0.2.2:1234"
	w := httptest.NewRecorder()
	second, _ := h.registerSubscriber(t.Context(), w, cookieRequest)
	assert.Nil(t, second)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	otherRequest := httptest.NewRequest(http.MethodGet, defaultHubURL+"?match=foo", nil)
	otherRequest.Header.Set("Authorization", bearerPrefix+createDummyAuthorizedJWT(roleSubscriber, []string{"foo"}))
	other, _ := h.registerSubscriber(t.Context(), httptest.NewRecorder(), otherRequest)
	require.NotNil(t, other)
	h.shutdown(t.Context(), other)
}
