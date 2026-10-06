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
	hubNameKeyType              struct{}
)

var (
	SubscriptionsContextKey           = subscriptionsKeyType{}        //nolint:gochecknoglobals
	WriteTimeoutContextKey            = writeTimeoutKeyType{}         //nolint:gochecknoglobals
	SubscriberListCacheSizeContextKey = subscriberListCacheSizeType{} //nolint:gochecknoglobals
	// HubNameContextKey carries the hub name: pooled transports must key by it
	// so that differently named hubs never share subscribers or history.
	HubNameContextKey = hubNameKeyType{} //nolint:gochecknoglobals
)

func hubName(ctx caddy.Context) string {
	name, _ := ctx.Value(HubNameContextKey).(string)

	return name
}
