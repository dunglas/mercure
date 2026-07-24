package mercure

import (
	"context"
	"net/http"
)

// requestOriginKey keys the per-request origin resolved by an embedding server.
type requestOriginKey struct{}

// RequestOrigin is the scheme and host an embedding server resolved for the
// current request. The hub derives its per-request OAuth resource identifier
// and RFC 9728 protected resource metadata URL from it (see requestIdentity).
type RequestOrigin struct {
	Scheme string
	Host   string
}

// NewRequestOriginContext returns a copy of ctx carrying the resolved request
// origin. An embedding server (for example the Caddy module) calls it once per
// request before invoking the hub, passing scheme and host taken from a trusted
// source; the hub validates them against the public-URL allowlist. The Caddy
// module reads them from Caddy's request placeholders so the values honor the
// trusted_proxies configuration rather than raw forwarded headers.
func NewRequestOriginContext(ctx context.Context, scheme, host string) context.Context {
	return context.WithValue(ctx, requestOriginKey{}, RequestOrigin{Scheme: scheme, Host: host})
}

// requestOrigin returns the origin an embedding server resolved for r, falling
// back to the request's own scheme and Host for standalone net/http use.
func (h *Hub) requestOrigin(r *http.Request) (scheme, host string) {
	if o, ok := r.Context().Value(requestOriginKey{}).(RequestOrigin); ok {
		return o.Scheme, o.Host
	}

	scheme = "http"
	if r.TLS != nil {
		scheme = schemeHTTPS
	}

	return scheme, r.Host
}
