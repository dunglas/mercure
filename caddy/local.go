package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/mercure"
)

type localTransportKey struct {
	hub string
}

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(&Local{})
}

type Local struct {
	transport *mercure.LocalTransport
	key       localTransportKey
}

// CaddyModule returns the Caddy module information.
func (*Local) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.mercure.local",
		New: func() caddy.Module { return new(Local) },
	}
}

func (l *Local) GetTransport() mercure.Transport { //nolint:ireturn
	return l.transport
}

// Provision provisions l's configuration.
func (l *Local) Provision(ctx caddy.Context) error {
	l.key = localTransportKey{hubName(ctx)}

	destructor, _, _ := TransportUsagePool.LoadOrNew(l.key, func() (caddy.Destructor, error) {
		return TransportDestructor[*mercure.LocalTransport]{
			Transport: mercure.NewLocalTransport(
				mercure.NewSubscriberList(ctx.Value(SubscriberListCacheSizeContextKey).(int)),
			),
		}, nil
	})

	l.transport = destructor.(TransportDestructor[*mercure.LocalTransport]).Transport

	return nil
}

//nolint:wrapcheck
func (l *Local) Cleanup() error {
	_, err := TransportUsagePool.Delete(l.key)

	return err
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
func (l *Local) UnmarshalCaddyfile(_ *caddyfile.Dispenser) error {
	return nil
}

var (
	_ caddy.Provisioner     = (*Local)(nil)
	_ caddy.CleanerUpper    = (*Local)(nil)
	_ caddyfile.Unmarshaler = (*Local)(nil)
)
