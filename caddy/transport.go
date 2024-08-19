package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/mercure"
)

var transport = caddy.NewUsagePool() //nolint:gochecknoglobals

type Transport interface {
	GetTransport() mercure.Transport
}

type transportDestructor[T mercure.Transport] struct {
	transport T
}

func (d transportDestructor[T]) Destruct() error {
	return d.transport.Close() //nolint:wrapcheck
}
