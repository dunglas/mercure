package mercure

import (
	"crypto/sha256"
	"errors"
	"math/rand/v2"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
)

// SubscriptionLimits bounds concurrent HTTP event streams on one hub instance.
// A zero field disables that limit, and all are zero by default. Client limits
// use RemoteAddr, never untrusted forwarding headers; token limits count
// verified tokens.
type SubscriptionLimits struct {
	Total     int `json:"total"`
	PerToken  int `json:"per_token"`
	PerClient int `json:"per_client"`
}

// ErrInvalidSubscriptionLimits rejects negative subscription limits.
var ErrInvalidSubscriptionLimits = errors.New("subscription limits must be non-negative")

// WithSubscriptionLimits sets the concurrent HTTP subscription limits.
func WithSubscriptionLimits(limits SubscriptionLimits) Option {
	return func(o *opt) error {
		if limits.Total < 0 || limits.PerToken < 0 || limits.PerClient < 0 {
			return ErrInvalidSubscriptionLimits
		}

		o.subscriptionLimits = limits

		return nil
	}
}

type subscriptionLimiter struct {
	mutex   sync.Mutex
	limits  SubscriptionLimits
	total   int
	tokens  map[[sha256.Size]byte]int
	clients map[string]int
}

type subscriptionPermit struct {
	limiter    *subscriptionLimiter
	token      [sha256.Size]byte
	countToken bool
	client     string
	once       sync.Once
}

func (h *Hub) admitSubscription(r *http.Request, claims *claims) (*subscriptionPermit, string) {
	l := h.subscriptionLimiter
	p := &subscriptionPermit{limiter: l}

	if claims != nil && l.limits.PerToken > 0 {
		var token string

		if header := r.Header.Get("Authorization"); header != "" {
			token = header[len(bearerPrefix):]
		} else if legacy, ok := h.legacyAuthQueryParam(r); ok {
			token = legacy
		} else if cookie, err := h.readCookie(r); err == nil {
			token = cookie.Value
		}

		p.token = sha256.Sum256([]byte(token))
		p.countToken = true
	}

	if l.limits.PerClient > 0 {
		p.client = r.RemoteAddr
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			p.client = host
		}

		if addr, err := netip.ParseAddr(p.client); err == nil {
			p.client = addr.Unmap().String()

			// A single IPv6 host usually controls a whole /64.
			if addr.Is6() && !addr.Is4In6() {
				prefix, _ := addr.Prefix(64)
				p.client = prefix.String()
			}
		}
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	switch {
	case l.limits.Total > 0 && l.total >= l.limits.Total:
		return nil, "total"
	case p.countToken && l.tokens[p.token] >= l.limits.PerToken:
		return nil, "token"
	case l.limits.PerClient > 0 && l.clients[p.client] >= l.limits.PerClient:
		return nil, "client"
	}

	l.total++
	if p.countToken {
		l.tokens[p.token]++
	}

	if l.limits.PerClient > 0 {
		l.clients[p.client]++
	}

	return p, ""
}

func (p *subscriptionPermit) release() {
	if p == nil {
		return
	}

	p.once.Do(func() {
		l := p.limiter
		l.mutex.Lock()
		defer l.mutex.Unlock()

		l.total--
		if p.countToken {
			l.tokens[p.token]--
			if l.tokens[p.token] == 0 {
				delete(l.tokens, p.token)
			}
		}

		if l.limits.PerClient > 0 {
			l.clients[p.client]--
			if l.clients[p.client] == 0 {
				delete(l.clients, p.client)
			}
		}
	})
}

func (h *Hub) reserveSubscription(w http.ResponseWriter, r *http.Request, claims *claims) *subscriptionPermit {
	permit, reason := h.admitSubscription(r, claims)
	if permit != nil {
		return permit
	}

	if metrics, ok := h.metrics.(interface{ SubscriptionRejected(reason string) }); ok {
		metrics.SubscriptionRejected(reason)
	}

	// Jitter spreads retries from clients rejected together.
	w.Header().Set("Retry-After", strconv.Itoa(5+rand.IntN(6))) //nolint:gosec
	http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)

	return nil
}
