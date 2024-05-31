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

	"github.com/MicahParks/keyfunc/v3"
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
	ErrCompatibility = errors.New("compatibility mode only supports protocol version 7")
	transports       = caddy.NewUsagePool()                                       //nolint:gochecknoglobals
	metrics          = mercure.NewPrometheusMetrics(prometheus.DefaultRegisterer) //nolint:gochecknoglobals
)

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(Mercure{})
	httpcaddyfile.RegisterHandlerDirective("mercure", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("mercure", "after", "encode")
}

type JWTConfig struct {
	Key string `json:"key,omitempty"`
	Alg string `json:"alg,omitempty"`
}

type transportDestructor struct {
	transport mercure.Transport
}

func (d *transportDestructor) Destruct() error {
	return d.transport.Close() //nolint:wrapcheck
}

// Mercure implements a Mercure hub as a Caddy module. Mercure is a protocol allowing to push data updates to web browsers and other HTTP clients in a convenient, fast, reliable and battery-efficient way.
type Mercure struct {
	// Allow subscribers with no valid JWT.
	Anonymous bool `json:"anonymous,omitempty"`

	// Dispatch updates when subscriptions are created or terminated
	Subscriptions bool `json:"subscriptions,omitempty"`

	// Enable the demo.
	Demo bool `json:"demo,omitempty"`

	// Enable the UI.
	UI bool `json:"ui,omitempty"`

	// Maximum duration before closing the connection, defaults to 600s, set to 0 to disable.
	WriteTimeout *caddy.Duration `json:"write_timeout,omitempty"`

	// Maximum dispatch duration of an update, defaults to 5s.
	DispatchTimeout *caddy.Duration `json:"dispatch_timeout,omitempty"`

	// Frequency of the heartbeat, defaults to 40s.
	Heartbeat *caddy.Duration `json:"heartbeat,omitempty"`

	// JWT key and signing algorithm to use for publishers.
	PublisherJWT JWTConfig `json:"publisher_jwt,omitempty"`

	// JWK Set URL to use for publishers.
	PublisherJWKSURL string `json:"publisher_jwks_url,omitempty"`

	// JWT key and signing algorithm to use for subscribers.
	SubscriberJWT JWTConfig `json:"subscriber_jwt,omitempty"`

	// JWK Set URL to use for subscribers.
	SubscriberJWKSURL string `json:"subscriber_jwks_url,omitempty"`

	// Origins allowed to publish updates
	PublishOrigins []string `json:"publish_origins,omitempty"`

	// Allowed CORS origins.
	CORSOrigins []string `json:"cors_origins,omitempty"`

	// Transport to use.
	TransportURL string `json:"transport_url,omitempty"`

	// Triggers use of LRU topic selector cache and avoidance of select priority queue (recommend 10,000 - 1,000,000)
	LRUShardSize *int64 `json:"lru_shard_size,omitempty"`

	// The name of the authorization cookie. Defaults to "mercureAuthorization".
	CookieName string `json:"cookie_name,omitempty"`

	// The version of the Mercure protocol to be backward compatible with (only version 7 is supported)
	ProtocolVersionCompatibility int `json:"protocol_version_compatibility,omitempty"`

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

func (m *Mercure) populateJWTConfig() error {
	repl := caddy.NewReplacer()

	if m.PublisherJWKSURL == "" {
		m.PublisherJWT.Key = repl.ReplaceKnown(m.PublisherJWT.Key, "")
		m.PublisherJWT.Alg = repl.ReplaceKnown(m.PublisherJWT.Alg, "HS256")

		if m.PublisherJWT.Key == "" {
			return errors.New("a JWT key or the URL of a JWK Set for publishers must be provided") //nolint:goerr113
		}

		if m.PublisherJWT.Alg == "" {
			m.PublisherJWT.Alg = "HS256"
		}
	}

	if m.SubscriberJWKSURL == "" {
		m.SubscriberJWT.Key = repl.ReplaceKnown(m.SubscriberJWT.Key, "")
		m.SubscriberJWT.Alg = repl.ReplaceKnown(m.SubscriberJWT.Alg, "HS256")

		if m.SubscriberJWT.Key == "" {
			if !m.Anonymous {
				return errors.New("a JWT key or the URL of a JWK Set for subscribers must be provided") //nolint:goerr113
			}
		}

		if m.SubscriberJWT.Alg == "" {
			m.SubscriberJWT.Alg = "HS256"
		}
	}

	return nil
}

func (m *Mercure) Provision(ctx caddy.Context) error { //nolint:funlen,gocognit
	if err := m.populateJWTConfig(); err != nil {
		return err
	}

	if m.TransportURL == "" {
		m.TransportURL = "bolt://mercure.db"
	}

	maxEntriesPerShard := mercure.DefaultTopicSelectorStoreLRUMaxEntriesPerShard
	if m.LRUShardSize != nil {
		maxEntriesPerShard = *m.LRUShardSize
	}

	tss, err := mercure.NewTopicSelectorStoreLRU(maxEntriesPerShard, mercure.DefaultTopicSelectorStoreLRUShardCount)
	if err != nil {
		return err //nolint:wrapcheck
	}

	m.logger = ctx.Logger(m)
	destructor, _, err := transports.LoadOrNew(m.TransportURL, func() (caddy.Destructor, error) {
		u, err := url.Parse(m.TransportURL)
		if err != nil {
			return nil, fmt.Errorf("invalid transport url: %w", err)
		}

		query := u.Query()
		if m.WriteTimeout != nil && !query.Has("write_timeout") {
			query.Set("write_timeout", time.Duration(*m.WriteTimeout).String())
		}

		if m.Subscriptions && !query.Has("subscriptions") {
			query.Set("subscriptions", "1")
		}

		u.RawQuery = query.Encode()

		transport, err := mercure.NewTransport(u, m.logger)
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
		mercure.WithCookieName(m.CookieName),
	}
	if m.logger.Core().Enabled(zapcore.DebugLevel) {
		opts = append(opts, mercure.WithDebug())
	}
	if m.PublisherJWKSURL == "" {
		opts = append(opts, mercure.WithPublisherJWT([]byte(m.PublisherJWT.Key), m.PublisherJWT.Alg))
	} else {
		k, err := keyfunc.NewDefaultCtx(ctx, []string{m.PublisherJWKSURL})
		if err != nil {
			return fmt.Errorf("failed to retrieve publisher JWK Set: %w", err)
		}

		opts = append(opts, mercure.WithPublisherJWTKeyFunc(k.Keyfunc))
	}

	if m.SubscriberJWKSURL != "" {
		k, err := keyfunc.NewDefaultCtx(ctx, []string{m.SubscriberJWKSURL})
		if err != nil {
			return fmt.Errorf("failed to retrieve subscriber JWK Set: %w", err)
		}

		opts = append(opts, mercure.WithSubscriberJWTKeyFunc(k.Keyfunc))
	} else if m.SubscriberJWT.Key != "" {
		opts = append(opts, mercure.WithSubscriberJWT([]byte(m.SubscriberJWT.Key), m.SubscriberJWT.Alg))
	}

	if m.Anonymous {
		opts = append(opts, mercure.WithAnonymous())
	}
	if m.Demo {
		opts = append(opts, mercure.WithDemo())
	}
	if m.UI {
		opts = append(opts, mercure.WithUI())
	}
	if m.Subscriptions {
		opts = append(opts, mercure.WithSubscriptions())
	}
	if d := m.WriteTimeout; d != nil {
		opts = append(opts, mercure.WithWriteTimeout(time.Duration(*d)))
	}
	if d := m.DispatchTimeout; d != nil {
		opts = append(opts, mercure.WithDispatchTimeout(time.Duration(*d)))
	}
	if d := m.Heartbeat; d != nil {
		opts = append(opts, mercure.WithHeartbeat(time.Duration(*d)))
	}
	if len(m.PublishOrigins) > 0 {
		opts = append(opts, mercure.WithPublishOrigins(m.PublishOrigins))
	}
	if len(m.CORSOrigins) > 0 {
		opts = append(opts, mercure.WithCORSOrigins(m.CORSOrigins))
	}
	if m.ProtocolVersionCompatibility != 0 {
		opts = append(opts, mercure.WithProtocolVersionCompatibility(m.ProtocolVersionCompatibility))
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
		return next.ServeHTTP(w, r) //nolint:wrapcheck
	}

	m.hub.ServeHTTP(w, r)

	return nil
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
func (m *Mercure) UnmarshalCaddyfile(d *caddyfile.Dispenser) error { //nolint:funlen,gocognit,gocyclo
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "anonymous":
				m.Anonymous = true

			case "demo":
				m.Demo = true

			case "ui":
				m.UI = true

			case "subscriptions":
				m.Subscriptions = true

			case "write_timeout":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				d, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err //nolint:wrapcheck
				}

				cd := caddy.Duration(d)
				m.WriteTimeout = &cd

			case "dispatch_timeout":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				d, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err //nolint:wrapcheck
				}

				cd := caddy.Duration(d)
				m.DispatchTimeout = &cd

			case "heartbeat":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				d, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err //nolint:wrapcheck
				}

				cd := caddy.Duration(d)
				m.Heartbeat = &cd

			case "publisher_jwks_url":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.PublisherJWKSURL = d.Val()

			case "publisher_jwt":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.PublisherJWT.Key = d.Val()
				if d.NextArg() {
					m.PublisherJWT.Alg = d.Val()
				}

			case "subscriber_jwks_url":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.SubscriberJWKSURL = d.Val()

			case "subscriber_jwt":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.SubscriberJWT.Key = d.Val()
				if d.NextArg() {
					m.SubscriberJWT.Alg = d.Val()
				}

			case "publish_origins":
				ra := d.RemainingArgs()
				if len(ra) == 0 {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.PublishOrigins = ra

			case "cors_origins":
				ra := d.RemainingArgs()
				if len(ra) == 0 {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.CORSOrigins = ra

			case "transport_url":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.TransportURL = d.Val()

			case "lru_cache":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				v, err := strconv.ParseInt(d.Val(), 10, 64)
				if err != nil {
					return err //nolint:wrapcheck
				}

				m.LRUShardSize = &v

			case "cookie_name":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				m.CookieName = d.Val()

			case "protocol_version_compatibility":
				if !d.NextArg() {
					return d.ArgErr() //nolint:wrapcheck
				}

				v, err := strconv.Atoi(d.Val())
				if err != nil {
					return err //nolint:wrapcheck
				}

				if v != 7 {
					return ErrCompatibility
				}

				m.ProtocolVersionCompatibility = v
			}
		}
	}

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) { //nolint:ireturn
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
