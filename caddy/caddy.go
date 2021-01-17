// Package caddy provides a handler for Caddy Server (https://caddyserver.com/)
// allowing to transform any Caddy instance into a Mercure hub.
package caddy

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/mercure"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const defaultHubURL = "/.well-known/mercure"

var (
	transports = caddy.NewUsagePool()                                       //nolint:gochecknoglobals
	metrics    = mercure.NewPrometheusMetrics(prometheus.DefaultRegisterer) //nolint:gochecknoglobals
)

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(Mercure{})
	httpcaddyfile.RegisterHandlerDirective("mercure", parseCaddyfile)
}

type JWTConfig struct {
	Key string `json:"key,omitempty"`
	Alg string `json:"alg,omitempty"`
}

type transportDestructor struct {
	transport mercure.Transport
}

func (d *transportDestructor) Destruct() error {
	return d.transport.Close()
}

type Mercure struct {
	// Allow subscribers with no valid JWT.
	Anonymous bool `json:"anonymous,omitempty"`

	// Dispatch updates when subscriptions are created or terminated
	Subscriptions bool `json:"subscriptions,omitempty"`

	// Enable the demo.
	Demo string `json:"demo,omitempty"`

	// Maximum duration before closing the connection, defaults to 600s, set to 0 to disable.
	WriteTimeout caddy.Duration `json:"write_timeout,omitempty"`

	// Maximum dispatch duration of an update.
	DispatchTimeout caddy.Duration `json:"dispatch_timeout,omitempty"`

	// Frequency of the heartbeat, defaults to 40s.
	Heartbeat caddy.Duration `json:"heartbeat,omitempty"`

	// JWT key and signing algorithm to use for publishers.
	PublisherJWT JWTConfig `json:"publisher_jwt,omitempty"`

	// JWT key and signing algorithm to use for subscribers.
	SubscriberJWT JWTConfig `json:"subscriber_jwt,omitempty"`

	// Origins allowed to publish updates
	PublishOrigins []string `json:"publish_origins,omitempty"`

	// Allowed CORS origins.
	CORSOrigins []string `json:"cors_origins,omitempty"`

	// Transport to use.
	TransportURL string `json:"transport_url,omitempty"`

	// Number of cache counters, defaults to 6e7, set to -1 to disable the cache. See https://github.com/dgraph-io/ristretto for details.
	CacheNumCounters *int64 `json:"cache_max_counters,omitempty"`

	// Maximum cache cost, defaults to 100MB, set to -1 to disable the cache. See https://github.com/dgraph-io/ristretto for details.
	CacheMaxCost *int64 `json:"cache_max_cost,omitempty"`

	hub    *mercure.Hub
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (Mercure) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.mercure",
		New: func() caddy.Module { return new(Mercure) },
	}
}

func (m *Mercure) Provision(ctx caddy.Context) error { //nolint:funlen
	repl := caddy.NewReplacer()

	m.PublisherJWT.Key = repl.ReplaceKnown(m.PublisherJWT.Key, "")
	m.PublisherJWT.Alg = repl.ReplaceKnown(m.PublisherJWT.Alg, "HS256")
	m.SubscriberJWT.Key = repl.ReplaceKnown(m.SubscriberJWT.Key, "")
	m.SubscriberJWT.Alg = repl.ReplaceKnown(m.SubscriberJWT.Alg, "HS256")

	if m.PublisherJWT.Key == "" {
		return errors.New("a JWT key for publishers must be provided") //nolint:goerr113
	}
	if m.PublisherJWT.Alg == "" {
		m.PublisherJWT.Alg = "HS256"
	}
	if m.TransportURL == "" {
		m.TransportURL = "bolt://mercure.db"
	}

	var (
		nc int64
		mc int64
	)
	if m.CacheNumCounters == nil {
		nc = mercure.TopicSelectorStoreDefaultCacheNumCounters
	} else {
		nc = *m.CacheNumCounters
	}

	if m.CacheMaxCost == nil {
		mc = mercure.TopicSelectorStoreCacheMaxCost
	} else {
		mc = *m.CacheMaxCost
	}

	tss, err := mercure.NewTopicSelectorStore(nc, mc)
	if err != nil {
		return err //nolint:wrapcheck
	}

	m.logger = ctx.Logger(m)
	destructor, _, err := transports.LoadOrNew(m.TransportURL, func() (caddy.Destructor, error) {
		u, err := url.Parse(m.TransportURL)
		if err != nil {
			return nil, fmt.Errorf("invalid transport url: %w", err)
		}

		if m.WriteTimeout != 0 {
			u.Query().Set("write_timeout", time.Duration(m.WriteTimeout).String())
		}

		transport, err := mercure.NewTransport(u, m.logger, tss)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}

		return &transportDestructor{transport}, nil
	})
	if err != nil {
		return err //nolint:wrapcheck
	}

	opts := []mercure.Option{
		mercure.WithLogger(m.logger),
		mercure.WithTopicSelectorStore(tss),
		mercure.WithTransport(destructor.(*transportDestructor).transport),
		mercure.WithMetrics(metrics),
		mercure.WithPublisherJWT([]byte(m.PublisherJWT.Key), m.PublisherJWT.Alg),
	}
	if m.logger.Core().Enabled(zapcore.DebugLevel) {
		opts = append(opts, mercure.WithDebug())
	}

	if m.SubscriberJWT.Key == "" {
		if !m.Anonymous {
			return errors.New("a JWT key for subscribers must be provided") //nolint:goerr113
		}
	} else {
		if m.SubscriberJWT.Alg == "" {
			m.SubscriberJWT.Alg = "HS256"
		}

		opts = append(opts, mercure.WithSubscriberJWT([]byte(m.SubscriberJWT.Key), m.SubscriberJWT.Alg))
	}

	if m.Anonymous {
		opts = append(opts, mercure.WithAnonymous())
	}
	if m.Demo != "" {
		opts = append(opts, mercure.WithDemo(m.Demo))
	}
	if m.Subscriptions {
		opts = append(opts, mercure.WithSubscriptions())
	}
	if d := m.WriteTimeout; d != 0 {
		opts = append(opts, mercure.WithWriteTimeout(time.Duration(d)))
	}
	if d := m.DispatchTimeout; d != 0 {
		opts = append(opts, mercure.WithDispatchTimeout(time.Duration(d)))
	}
	if d := m.Heartbeat; d != 0 {
		opts = append(opts, mercure.WithHeartbeat(time.Duration(d)))
	}
	if len(m.PublishOrigins) > 0 {
		opts = append(opts, mercure.WithPublishOrigins(m.PublishOrigins))
	}
	if len(m.CORSOrigins) > 0 {
		opts = append(opts, mercure.WithCORSOrigins(m.CORSOrigins))
	}

	h, err := mercure.NewHub(opts...)
	if err != nil {
		return err //nolint:wrapcheck
	}

	m.hub = h

	return nil
}

func (m *Mercure) Cleanup() error {
	_, err := transports.Delete(m.TransportURL)

	return err //nolint:wrapcheck
}

func (m Mercure) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if !strings.HasPrefix(r.URL.Path, defaultHubURL) {
		return next.ServeHTTP(w, r)
	}

	m.hub.ServeHTTP(w, r)

	return nil
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
func (m *Mercure) UnmarshalCaddyfile(d *caddyfile.Dispenser) error { //nolint:funlen nolint:gocognit
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "anonymous":
				m.Anonymous = true

			case "demo":
				if d.NextArg() {
					m.Demo = d.Val()
				} else {
					m.Demo = "public/"
				}

			case "subscriptions":
				m.Subscriptions = true

			case "write_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}

				d, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err //nolint:wrapcheck
				}

				m.WriteTimeout = caddy.Duration(d)

			case "dispatch_timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}

				d, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err //nolint:wrapcheck
				}

				m.DispatchTimeout = caddy.Duration(d)

			case "heartbeat":
				if !d.NextArg() {
					return d.ArgErr()
				}

				d, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err //nolint:wrapcheck
				}

				m.Heartbeat = caddy.Duration(d)

			case "publisher_jwt":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.PublisherJWT.Key = d.Val()
				if d.NextArg() {
					m.PublisherJWT.Alg = d.Val()
				}

			case "subscriber_jwt":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.SubscriberJWT.Key = d.Val()
				if d.NextArg() {
					m.SubscriberJWT.Alg = d.Val()
				}

			case "publish_origins":
				ra := d.RemainingArgs()
				if len(ra) == 0 {
					return d.ArgErr()
				}

				m.PublishOrigins = ra

			case "cors_origins":
				ra := d.RemainingArgs()
				if len(ra) == 0 {
					return d.ArgErr()
				}

				m.CORSOrigins = ra

			case "transport_url":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.TransportURL = d.Val()

			case "cache":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.ParseInt(d.Val(), 10, 64)
				if err != nil {
					return err //nolint:wrapcheck
				}

				m.CacheNumCounters = &v

				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err = strconv.ParseInt(d.Val(), 10, 64)
				if err != nil {
					return err //nolint:wrapcheck
				}

				m.CacheMaxCost = &v
			}
		}
	}

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m Mercure
	err := m.UnmarshalCaddyfile(h.Dispenser)

	return m, err
}

// Interface guards.
var (
	_ caddy.Provisioner           = (*Mercure)(nil)
	_ caddy.CleanerUpper          = (*Mercure)(nil)
	_ caddyhttp.MiddlewareHandler = (*Mercure)(nil)
	_ caddyfile.Unmarshaler       = (*Mercure)(nil)
)
