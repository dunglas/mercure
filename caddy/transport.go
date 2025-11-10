package caddy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/mercure"
)

var TransportUsagePool = caddy.NewUsagePool() //nolint:gochecknoglobals

type Transport interface {
	GetTransport() mercure.Transport
}

type TransportDestructor[T mercure.Transport] struct {
	Transport T
}

func (d TransportDestructor[T]) Destruct() error {
	return d.Transport.Close(caddy.ActiveContext()) //nolint:wrapcheck
}

type (
	subscriptionsKeyType        struct{}
	writeTimeoutKeyType         struct{}
	subscriberListCacheSizeType struct{}
)

var (
	SubscriptionsContextKey           = subscriptionsKeyType{}        //nolint:gochecknoglobals
	WriteTimeoutContextKey            = writeTimeoutKeyType{}         //nolint:gochecknoglobals
	SubscriberListCacheSizeContextKey = subscriberListCacheSizeType{} //nolint:gochecknoglobals
)
