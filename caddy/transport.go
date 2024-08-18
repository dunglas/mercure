package caddy

import "github.com/caddyserver/caddy/v2"

// ContextAwareTransport can be implemented by transports to access the Caddy context.
type ContextAwareTransport interface {
	SetContext(ctx caddy.Context) error
}
