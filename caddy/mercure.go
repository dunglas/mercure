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
	// AllowNoPublish allows not setting the publisher JWT, and then disable the publish endpoint.
	//
	// EXPERIMENTAL.
	//
	// It is usually set to true in the init() function of Go applications allowing to publish programmatically by
	// calling mercure.Publish() directly.
	AllowNoPublish bool //nolint:gochecknoglobals

	ErrCompatibility = errors.New("compatibility mode only supports protocol versions 7 and 8")

	// hubs is a list of registered Mercure hubs, the key is the top-most subroute.
	hubs   = make(map[caddy.Module]*hubInfo) //nolint:gochecknoglobals
	hubsMu sync.Mutex                        //nolint:gochecknoglobals
)

type hubInfo struct {
	hub       *mercure.Hub
	transport mercure.Transport
	name      string
}

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(&Mercure{})
	httpcaddyfile.RegisterHandlerDirective("mercure", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("mercure", "after", "encode")
}

// FindHub finds the Mercure hub configured for the current route.
//
// EXPERIMENTAL.
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

// Mercure implements a Mercure hub as a Caddy module. Mercure is a protocol allowing to push data updates to web browsers and other HTTP clients in a convenient, fast, reliable and battery-efficient way.
type Mercure struct {
	deprecatedTransport

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

	// Allowed JWS algorithms for publisher tokens validated via the JWK Set
	// (RFC 8725: the algorithm is never taken from the token header). When
	// omitted in modern mode, the hub applies its default allowlist of
	// asymmetric algorithms.
	PublisherJWKSAlgorithms []string `json:"publisher_jwks_algorithms,omitempty"`

	// JWT key and signing algorithm to use for subscribers.
	SubscriberJWT JWTConfig `json:"subscriber_jwt,omitzero"`

	// JWK Set URL to use for subscribers.
	SubscriberJWKSURL string `json:"subscriber_jwks_url,omitempty"`

	// Allowed JWS algorithms for subscriber tokens validated via the JWK Set.
	// See PublisherJWKSAlgorithms.
	SubscriberJWKSAlgorithms []string `json:"subscriber_jwks_algorithms,omitempty"`

	// Origins allowed to publish updates
	PublishOrigins []string `json:"publish_origins,omitempty"`

	// Allowed CORS origins.
	CORSOrigins []string `json:"cors_origins,omitempty"`

	// Maximum number of entries in the topic matcher cache. 0 or negative
	// disables the cache. Defaults to DefaultTopicMatcherStoreCacheSize.
	TopicMatcherCacheSize *int `json:"topic_matcher_cache_size,omitempty"`

	SubscriberListCacheSize *int `json:"subscriber_list_cache_size,omitempty"`

	// The name of the authorization cookie. Defaults to "mercure_access_token".
	CookieName string `json:"cookie_name,omitempty"`

	// The URL at which subscribers reach the hub. Used as the base URL when
	// matching relative URL patterns and topics.
	PublicURL string `json:"public_url,omitempty"`

	// The hub's OAuth 2.0 resource identifier (the `aud` value access tokens
	// must carry). Defaults to the public URL when unset.
	ResourceIdentifier string `json:"resource_identifier,omitempty"`

	// OAuth 2.0 authorization server issuer identifiers advertised in the
	// hub's protected resource metadata.
	AuthorizationServers []string `json:"authorization_servers,omitempty"`

	// Issuer identifiers accepted in the token iss claim in addition to the
	// authorization servers, for self-issued tokens (RFC 9068).
	TrustedIssuers []string `json:"trusted_issuers,omitempty"`

	// The version of the Mercure protocol to be backward compatible with (versions 7 and 8 are supported)
	ProtocolVersionCompatibility int `json:"protocol_version_compatibility,omitempty"`

	// The transport configuration.
	TransportRaw json.RawMessage `json:"transport,omitempty" caddy:"namespace=http.handlers.mercure inline_key=name"` //nolint:tagalign

	hub    *mercure.Hub
	logger *slog.Logger
	cancel context.CancelFunc
}

// CaddyModule returns the Caddy module information.
func (*Mercure) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.mercure",
		New: func() caddy.Module { return new(Mercure) },
	}
}

type stoppingHandlerFunc func()

func (s stoppingHandlerFunc) Handle(_ context.Context, _ caddy.Event) error {
	s()

	return nil
}

//nolint:wrapcheck
func (m *Mercure) Provision(ctx caddy.Context) (err error) { //nolint:funlen,gocognit,gocyclo,maintidx
	metrics := mercure.NewPrometheusMetrics(ctx.GetMetricsRegistry())

	if err := m.populateJWTConfig(); err != nil {
		return err
	}

	cacheSize := mercure.DefaultTopicMatcherStoreCacheSize
	if m.TopicMatcherCacheSize != nil {
		cacheSize = *m.TopicMatcherCacheSize
	}

	tms, err := mercure.NewTopicMatcherStore(cacheSize)
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
		mercure.WithTopicMatcherStore(tms),
		mercure.WithTransport(transport),
		mercure.WithMetrics(metrics),
		mercure.WithCookieName(m.CookieName),
		mercure.WithPublicURL(m.PublicURL),
	}

	if m.ResourceIdentifier != "" {
		opts = append(opts, mercure.WithResourceIdentifier(m.ResourceIdentifier))
	}

	if len(m.AuthorizationServers) > 0 {
		opts = append(opts, mercure.WithAuthorizationServers(m.AuthorizationServers))
	}

	if len(m.TrustedIssuers) > 0 {
		opts = append(opts, mercure.WithTrustedIssuers(m.TrustedIssuers))
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

		if len(m.PublisherJWKSAlgorithms) > 0 {
			opts = append(opts, mercure.WithPublisherJWTAlgorithms(m.PublisherJWKSAlgorithms))
		}
	} else if m.PublisherJWT.Key != "" {
		opts = append(opts, mercure.WithPublisherJWT([]byte(m.PublisherJWT.Key), m.PublisherJWT.Alg))
	}

	if m.SubscriberJWKSURL != "" {
		k, err := newJWKSetKeyfunc(ctx, m.SubscriberJWKSURL)
		if err != nil {
			return fmt.Errorf("failed to retrieve subscriber JWK Set: %w", err)
		}

		opts = append(opts, mercure.WithSubscriberJWTKeyFunc(k.Keyfunc))

		if len(m.SubscriberJWKSAlgorithms) > 0 {
			opts = append(opts, mercure.WithSubscriberJWTAlgorithms(m.SubscriberJWKSAlgorithms))
		}
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

func (m *Mercure) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if !strings.HasPrefix(r.URL.Path, defaultHubURL) && r.URL.Path != mercure.ProtectedResourceMetadataPath {
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

			case "publisher_jwks_algorithms":
				m.PublisherJWKSAlgorithms = d.RemainingArgs()
				if len(m.PublisherJWKSAlgorithms) == 0 {
					return d.ArgErr()
				}

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

			case "subscriber_jwks_algorithms":
				m.SubscriberJWKSAlgorithms = d.RemainingArgs()
				if len(m.SubscriberJWKSAlgorithms) == 0 {
					return d.ArgErr()
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

			case "topic_matcher_cache":
				if !d.NextArg() {
					return d.ArgErr()
				}

				size, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.WrapErr(err)
				}

				m.TopicMatcherCacheSize = &size
			case "subscriber_list_cache_size":
				if !d.NextArg() {
					return d.ArgErr()
				}

				size, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.WrapErr(err)
				}

				if size < 0 {
					return d.Errf("subscriber_list_cache_size must be >= 0, got %d", size)
				}

				m.SubscriberListCacheSize = &size

			case "cookie_name":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.CookieName = d.Val()

			case "public_url":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.PublicURL = d.Val()

			case "resource_identifier":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.ResourceIdentifier = d.Val()

			case "authorization_servers":
				m.AuthorizationServers = d.RemainingArgs()
				if len(m.AuthorizationServers) == 0 {
					return d.ArgErr()
				}

			case "trusted_issuers":
				m.TrustedIssuers = d.RemainingArgs()
				if len(m.TrustedIssuers) == 0 {
					return d.ArgErr()
				}

			case "protocol_version_compatibility":
				if !d.NextArg() {
					return d.ArgErr()
				}

				v, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.WrapErr(err)
				}

				if v != 7 && v != 8 {
					return d.WrapErr(ErrCompatibility)
				}

				m.ProtocolVersionCompatibility = v
			}
		}
	}

	m.assignDeprecatedTransportURLForEnv()

	return nil
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
//
//nolint:ireturn
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

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) { //nolint:ireturn
	m := new(Mercure)

	return m, m.UnmarshalCaddyfile(h.Dispenser)
}

func parseDurationParameter(d *caddyfile.Dispenser) (*caddy.Duration, error) {
	if !d.NextArg() {
		return nil, d.ArgErr() //nolint:wrapcheck
	}

	du, err := caddy.ParseDuration(d.Val())
	if err != nil {
		return nil, d.WrapErr(err) //nolint:wrapcheck
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
