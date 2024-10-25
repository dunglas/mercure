package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/mercure"
)

type localTransportKeyStruct struct{}

var localTransportKey = localTransportKeyStruct{} //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(Local{})
}

type Local struct {
	transport *mercure.LocalTransport
}

// CaddyModule returns the Caddy module information.
func (Local) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.mercure.local",
		New: func() caddy.Module { return new(Local) },
	}
}

func (l *Local) GetTransport() mercure.Transport { //nolint:ireturn
	return l.transport
}

// Provision provisions b's configuration.
func (l Local) Provision(_ caddy.Context) error {
	destructor, _, _ := transport.LoadOrNew(localTransportKey, func() (caddy.Destructor, error) {
		return transportDestructor[*mercure.LocalTransport]{transport: mercure.NewLocalTransport()}, nil
	})

	l.transport = destructor.(transportDestructor[*mercure.LocalTransport]).transport

	return nil
}

//nolint:wrapcheck
func (l *Local) Cleanup() error {
	_, err := transport.Delete(localTransportKey)

	return err
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
func (l *Local) UnmarshalCaddyfile(_ *caddyfile.Dispenser) error {
	return nil
}

var (
	_ caddy.Provisioner     = (*Bolt)(nil)
	_ caddy.CleanerUpper    = (*Bolt)(nil)
	_ caddyfile.Unmarshaler = (*Bolt)(nil)
)
