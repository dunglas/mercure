// Package caddy provides a handler for Caddy Server (https://caddyserver.com/)
// allowing to transform any Caddy instance into a Mercure hub.
package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyevents"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/dunglas/mercure"
)

const defaultHubURL = "/.well-known/mercure"

var (
	// EXPERIMENTAL: AllowNoPublish allows not setting the publisher JWT, and then disable the publish endpoint.
	//
	// It is usually set to true in the init() function of Go applications allowing to publish programmatically by
	// calling mercure.Publish() directly.
	AllowNoPublish bool //nolint:gochecknoglobals

	ErrCompatibility = errors.New("compatibility mode only supports protocol version 7")

	// hubs is a list of registered Mercure hubs, the key is the top-most subroute.
	hubs   = make(map[caddy.Module]*hubInfo) //nolint:gochecknoglobals
	hubsMu sync.Mutex
)

type hubInfo struct {
	hub       *mercure.Hub
	transport mercure.Transport
	name      string
}

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(Mercure{})
	httpcaddyfile.RegisterHandlerDirective("mercure", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("mercure", "after", "encode")
}

// EXPERIMENTAL: FindHub finds the Mercure hub configured for the current route.
func FindHub(modules []caddy.Module) *mercure.Hub {
	hubsMu.Lock()
	defer hubsMu.Unlock()

	for _, m := range modules {
		if info, ok := hubs[m]; ok {
			return info.hub
		}
	}

	if info := hubs[nil]; info != nil {
		return info.hub
	}

	return nil
}

type JWTConfig struct {
	Key string `json:"key,omitempty"`
	Alg string `json:"alg,omitempty"`
}

type TopicSelectorCacheConfig struct {
	// Deprecated: use Size instead.
	MaxEntriesPerShard int `json:"max_entries_per_shard,omitempty"`
	// Deprecated: no longer used.
	ShardCount uint64 `json:"shard_count,omitempty"`
	// Size is the maximum number of entries in the cache.
	Size int `json:"size,omitempty"`
}

// Mercure implements a Mercure hub as a Caddy module. Mercure is a protocol allowing to push data updates to web browsers and other HTTP clients in a convenient, fast, reliable and battery-efficient way.
type Mercure struct {
	// Human-readable name for this hub, used in health check endpoints and metrics.
	Name string `json:"name,omitempty"`

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
	PublisherJWT JWTConfig `json:"publisher_jwt,omitzero"`

	// JWK Set URL to use for publishers.
	PublisherJWKSURL string `json:"publisher_jwks_url,omitempty"`

	// JWT key and signing algorithm to use for subscribers.
	SubscriberJWT JWTConfig `json:"subscriber_jwt,omitzero"`

	// JWK Set URL to use for subscribers.
	SubscriberJWKSURL string `json:"subscriber_jwks_url,omitempty"`

	// Origins allowed to publish updates
	PublishOrigins []string `json:"publish_origins,omitempty"`

	// Allowed CORS origins.
	CORSOrigins []string `json:"cors_origins,omitempty"`

	// Deprecated: not used anymore.
	CacheShardSize *int64 `json:"cache_shard_size,omitempty"`

	// Triggers use of topic selector cache and avoidance of select priority queue.
	TopicSelectorCache *TopicSelectorCacheConfig `json:"cache,omitempty"`

	SubscriberListCacheSize *int `json:"subscriber_list_cache_size,omitempty"`

	// The name of the authorization cookie. Defaults to "mercureAuthorization".
	CookieName string `json:"cookie_name,omitempty"`

	// The version of the Mercure protocol to be backward compatible with (only version 7 is supported)
	ProtocolVersionCompatibility int `json:"protocol_version_compatibility,omitempty"`

	// The transport configuration.
	TransportRaw json.RawMessage `json:"transport,omitempty" caddy:"namespace=http.handlers.mercure inline_key=name"` //nolint:tagalign

	deprecatedTransport

	hub    *mercure.Hub
	logger *slog.Logger
	cancel context.CancelFunc
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

		if m.PublisherJWT.Key != "" {
			m.PublisherJWT.Alg = repl.ReplaceKnown(m.PublisherJWT.Alg, "HS256")
			if m.PublisherJWT.Alg == "" {
				m.PublisherJWT.Alg = "HS256"
			}
		} else if !AllowNoPublish {
			return errors.New("a JWT key or the URL of a JWK Set for publishers must be provided") //nolint:err113
		}
	}

	if m.SubscriberJWKSURL == "" {
		m.SubscriberJWT.Key = repl.ReplaceKnown(m.SubscriberJWT.Key, "")
		m.SubscriberJWT.Alg = repl.ReplaceKnown(m.SubscriberJWT.Alg, "HS256")

		if m.SubscriberJWT.Key == "" {
			if !m.Anonymous {
				return errors.New("a JWT key or the URL of a JWK Set for subscribers must be provided") //nolint:err113
			}
		}

		if m.SubscriberJWT.Alg == "" {
			m.SubscriberJWT.Alg = "HS256"
		}
	}

	return nil
}

// newJWKSetKeyfunc builds a Keyfunc from a JWK Set URL.
//
// file:// URLs point to a local JSON file containing a JWK Set; the file is
// read once at provision time, so rotating the keys requires a Caddy config
// reload. Other URLs are forwarded to keyfunc.NewDefaultCtx, which handles
// HTTP(S) and rejects unsupported schemes.
func newJWKSetKeyfunc(ctx context.Context, rawURL string) (keyfunc.Keyfunc, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid JWK Set URL %q: %w", rawURL, err)
	}

	if u.Scheme == "file" {
		if u.Host != "" && u.Host != "localhost" {
			return nil, fmt.Errorf(`file:// JWK Set URL host must be empty or "localhost", got %q`, u.Host) //nolint:err113
		}

		b, err := os.ReadFile(u.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read JWK Set file %q: %w", u.Path, err)
		}

		k, err := keyfunc.NewJWKSetJSON(b)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JWK Set file %q: %w", u.Path, err)
		}

		return k, nil
	}

	return keyfunc.NewDefaultCtx(ctx, []string{rawURL}) //nolint:wrapcheck
}

type stoppingHandlerFunc func()

func (s stoppingHandlerFunc) Handle(_ context.Context, _ caddy.Event) error {
	s()

	return nil
}

//nolint:wrapcheck
func (m *Mercure) Provision(ctx caddy.Context) (err error) { //nolint:funlen,gocognit
	metrics := mercure.NewPrometheusMetrics(ctx.GetMetricsRegistry())

	if err := m.populateJWTConfig(); err != nil {
		return err
	}

	cacheSize := mercure.DefaultTopicSelectorStoreCacheSize

	if m.TopicSelectorCache != nil {
		switch {
		case m.TopicSelectorCache.Size > 0:
			cacheSize = m.TopicSelectorCache.Size
		case m.TopicSelectorCache.MaxEntriesPerShard > 0:
			// Backward compat: convert old per-shard config
			shardCount := m.TopicSelectorCache.ShardCount
			if shardCount == 0 {
				shardCount = 256
			}

			cacheSize = m.TopicSelectorCache.MaxEntriesPerShard * int(shardCount)
		case m.TopicSelectorCache.MaxEntriesPerShard < 0:
			cacheSize = 0
		}
	}

	tss, err := mercure.NewTopicSelectorStore(cacheSize)
	if err != nil {
		return err
	}

	ctx = ctx.WithValue(SubscriptionsContextKey, m.Subscriptions)
	ctx = ctx.WithValue(WriteTimeoutContextKey, m.WriteTimeout)

	if m.SubscriberListCacheSize == nil {
		ctx = ctx.WithValue(SubscriberListCacheSizeContextKey, mercure.DefaultSubscriberListCacheSize)
	} else {
		ctx = ctx.WithValue(SubscriberListCacheSizeContextKey, *m.SubscriberListCacheSize)
	}

	m.logger = slog.New(mercure.NewSlogHandler(ctx.Slogger().Handler()))

	var transport mercure.Transport
	if transport, err = m.createTransportDeprecated(); err != nil {
		return err
	}

	if transport == nil {
		var mod any
		if m.TransportRaw == nil {
			mod, err = ctx.LoadModuleByID("http.handlers.mercure.bolt", nil)
		} else {
			mod, err = ctx.LoadModule(m, "TransportRaw")
		}

		if err != nil {
			return err
		}

		transport = mod.(Transport).GetTransport()
	}

	opts := []mercure.Option{
		mercure.WithLogger(m.logger),
		mercure.WithTopicSelectorStore(tss),
		mercure.WithTransport(transport),
		mercure.WithMetrics(metrics),
		mercure.WithCookieName(m.CookieName),
	}

	if m.logger.Enabled(ctx, slog.LevelDebug) {
		opts = append(opts, mercure.WithDebug())
	}

	if m.PublisherJWKSURL != "" {
		k, err := newJWKSetKeyfunc(ctx, m.PublisherJWKSURL)
		if err != nil {
			return fmt.Errorf("failed to retrieve publisher JWK Set: %w", err)
		}

		opts = append(opts, mercure.WithPublisherJWTKeyFunc(k.Keyfunc))
	} else if m.PublisherJWT.Key != "" {
		opts = append(opts, mercure.WithPublisherJWT([]byte(m.PublisherJWT.Key), m.PublisherJWT.Alg))
	}

	if m.SubscriberJWKSURL != "" {
		k, err := newJWKSetKeyfunc(ctx, m.SubscriberJWKSURL)
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

	eventApp, err := ctx.App("events")
	if err != nil {
		return err
	}

	var c context.Context
	c, m.cancel = context.WithCancel(ctx)
	if err := eventApp.(*caddyevents.App).On("stopping", stoppingHandlerFunc(m.cancel)); err != nil {
		return err
	}

	h, err := mercure.NewHub(c, opts...)
	if err != nil {
		return err
	}

	m.hub = h

	name := m.Name
	if name == "" {
		name = "default"
	}

	info := &hubInfo{
		hub:       h,
		transport: transport,
		name:      name,
	}

	var found bool

	hubsMu.Lock()
	defer hubsMu.Unlock()

	for _, m := range ctx.Modules() {
		if _, ok := m.(*caddyhttp.Subroute); ok {
			hubs[m] = info
			found = true

			break
		}
	}

	if !found {
		hubs[nil] = info
	}

	return nil
}

func (m *Mercure) Cleanup() error {
	if m.cancel != nil {
		m.cancel()
	}

	hubsMu.Lock()
	defer hubsMu.Unlock()

	for k, info := range hubs {
		if info.hub == m.hub {
			delete(hubs, k)
		}
	}

	return m.cleanupTransportDeprecated()
}

func (m Mercure) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if !strings.HasPrefix(r.URL.Path, defaultHubURL) {
		return next.ServeHTTP(w, r) //nolint:wrapcheck
	}

	m.hub.ServeHTTP(w, r)

	return nil
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens.
//
//nolint:wrapcheck
func (m *Mercure) UnmarshalCaddyfile(d *caddyfile.Dispenser) (err error) { //nolint:maintidx,funlen,gocognit,gocyclo
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "name":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.Name = d.Val()

			case "anonymous":
				m.Anonymous = true

			case "demo":
				m.Demo = true

			case "ui":
				m.UI = true

			case "subscriptions":
				m.Subscriptions = true

			case "write_timeout":
				if m.WriteTimeout, err = parseDurationParameter(d); err != nil {
					return err
				}

			case "dispatch_timeout":
				if m.DispatchTimeout, err = parseDurationParameter(d); err != nil {
					return err
				}

			case "heartbeat":
				if m.Heartbeat, err = parseDurationParameter(d); err != nil {
					return err
				}

			case "publisher_jwks_url":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.PublisherJWKSURL = d.Val()

			case "publisher_jwt":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.PublisherJWT.Key = d.Val()
				if d.NextArg() {
					m.PublisherJWT.Alg = d.Val()
				}

			case "subscriber_jwks_url":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.SubscriberJWKSURL = d.Val()

			case "subscriber_jwt":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.SubscriberJWT.Key = d.Val()
				if d.NextArg() {
					m.SubscriberJWT.Alg = d.Val()
				}

			case "publish_origins":
				m.PublishOrigins = d.RemainingArgs()
				if len(m.PublishOrigins) == 0 {
					return d.ArgErr()
				}

			case "cors_origins":
				m.CORSOrigins = d.RemainingArgs()
				if len(m.CORSOrigins) == 0 {
					return d.ArgErr()
				}

			case "transport":
				if !d.NextArg() {
					return d.ArgErr()
				}

				name := d.Val()
				modID := "http.handlers.mercure." + name

				unm, err := caddyfile.UnmarshalModule(d, modID)
				if err != nil {
					return err
				}

				t, ok := unm.(Transport)
				if !ok {
					return d.Errf(`module %s (%T) is not a supported transport implementation (requires "github.com/dunglas/mercure/caddy".Transport)`, modID, unm)
				}

				m.TransportRaw = caddyconfig.JSONModuleObject(t, "name", name, nil)

			case "transport_url":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.assignDeprecatedTransportURL(d.Val())

			case "topic_selector_cache":
				if !d.NextArg() {
					return d.ArgErr()
				}

				size, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.WrapErr(err)
				}

				m.TopicSelectorCache = &TopicSelectorCacheConfig{Size: size}
			case "subscriber_list_cache_size":
				if !d.NextArg() {
					return d.ArgErr()
				}

				s, err := strconv.ParseUint(d.Val(), 10, 64)
				if err != nil {
					return d.WrapErr(err)
				}

				size := int(s)
				m.SubscriberListCacheSize = &size

			case "cookie_name":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.CookieName = d.Val()

			case "protocol_version_compatibility":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.WrapErr(err)
				}

				if v != 7 {
					return d.WrapErr(ErrCompatibility)
				}

				m.ProtocolVersionCompatibility = v
			}
		}
	}

	m.assignDeprecatedTransportURLForEnv()

	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) { //nolint:ireturn
	var m Mercure

	return m, m.UnmarshalCaddyfile(h.Dispenser)
}

func parseDurationParameter(d *caddyfile.Dispenser) (*caddy.Duration, error) {
	if !d.NextArg() {
		return nil, d.ArgErr()
	}

	du, err := caddy.ParseDuration(d.Val())
	if err != nil {
		return nil, d.WrapErr(err)
	}

	cd := caddy.Duration(du)

	return &cd, nil
}

// Interface guards.
var (
	_ caddy.Provisioner           = (*Mercure)(nil)
	_ caddy.CleanerUpper          = (*Mercure)(nil)
	_ caddyhttp.MiddlewareHandler = (*Mercure)(nil)
	_ caddyfile.Unmarshaler       = (*Mercure)(nil)
)
