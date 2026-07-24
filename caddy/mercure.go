// Package caddy provides a handler for Caddy Server (https://caddyserver.com/)
// allowing to transform any Caddy instance into a Mercure hub.
package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
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
	"github.com/dustin/go-humanize"
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

// IssuerConfig binds a trusted issuer to its per-role verification material.
type IssuerConfig struct {
	// Identifier is the exact token iss claim value (RFC 9068 §4).
	Identifier string `json:"identifier,omitempty"`

	// AuthorizationServer advertises this issuer in the hub's RFC 9728
	// protected resource metadata. Leave false for self-issued tokens.
	AuthorizationServer bool `json:"authorization_server,omitempty"`

	// Publisher verifies publisher tokens from this issuer.
	Publisher VerifierConfig `json:"publisher,omitzero"`

	// Subscriber verifies subscriber tokens from this issuer.
	Subscriber VerifierConfig `json:"subscriber,omitzero"`
}

// VerifierConfig configures how one role's tokens are verified: either a static
// key (JWT) or a JWK Set (JWKSURL). The two are mutually exclusive.
type VerifierConfig struct {
	// JWT is a static key and its signing algorithm.
	JWT JWTConfig `json:"jwt,omitzero"`

	// JWKSURL is a JWK Set URL (the RFC 8414 jwks_uri member).
	JWKSURL string `json:"jwks_uri,omitempty"`

	// JWKSAlgorithms pins the allowed JWS algorithms for the JWK Set path
	// (RFC 8725). Defaults to the hub's asymmetric allowlist when empty.
	JWKSAlgorithms []string `json:"jwks_algorithms,omitempty"`
}

// isSet reports whether the verifier declares any material.
func (v VerifierConfig) isSet() bool {
	return v.JWT.Key != "" || v.JWKSURL != ""
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

	// Maximum size in bytes of publish and QUERY subscribe request bodies;
	// larger requests are rejected with a 413 status code. Defaults to 1MiB,
	// set to 0 to disable the in-hub limit.
	MaxRequestBodySize *int64 `json:"max_request_body_size,omitempty"`

	// Issuers binds each trusted issuer (RFC 9068 §4) to its own verification
	// material, so key material is never pooled across issuers.
	Issuers []IssuerConfig `json:"issuers,omitempty"`

	// Deprecated: use Issuers. Static publisher key and signing algorithm,
	// mapped to a single implicit issuer (usable only in compatibility mode).
	PublisherJWT JWTConfig `json:"publisher_jwt,omitzero"`

	// Deprecated: use Issuers. Publisher JWK Set URL, mapped to a single
	// implicit issuer (usable only in compatibility mode).
	PublisherJWKSURL string `json:"publisher_jwks_url,omitempty"`

	// Deprecated: use Issuers. Static subscriber key and signing algorithm,
	// mapped to a single implicit issuer (usable only in compatibility mode).
	SubscriberJWT JWTConfig `json:"subscriber_jwt,omitzero"`

	// Deprecated: use Issuers. Subscriber JWK Set URL, mapped to a single
	// implicit issuer (usable only in compatibility mode).
	SubscriberJWKSURL string `json:"subscriber_jwks_url,omitempty"`

	// Origins allowed to publish updates
	PublishOrigins []string `json:"publish_origins,omitempty"`

	// Allowed CORS origins.
	CORSOrigins []string `json:"cors_origins,omitempty"`

	// Maximum number of entries in the topic matcher cache. 0 or negative
	// disables the cache. Defaults to DefaultTopicMatcherStoreCacheSize.
	TopicMatcherCacheSize *int `json:"topic_matcher_cache_size,omitempty"`

	SubscriberListCacheSize *int `json:"subscriber_list_cache_size,omitempty"`

	// The name of the authorization cookie. Defaults to
	// "__Secure-mercure_access_token"; plain-HTTP deployments must configure a
	// prefix-less name.
	CookieName string `json:"cookie_name,omitempty"`

	// Public URLs the hub answers on. When set, a request whose origin (scheme
	// and host) is not listed is rejected with 421 Misdirected Request, pinning
	// the scheme too. Leave empty to rely on the site's own host matching; set
	// it for a catch-all site block that would otherwise let a client pick the
	// derived public URL.
	PublicURLs []string `json:"public_urls,omitempty"`

	// Pins the hub's OAuth 2.0 resource identifier (the `aud` value access
	// tokens must carry) to a single value. When unset, the hub derives it from
	// each request (the public URL the client contacted), so several public
	// URLs work without configuration; set it to force one canonical audience.
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

	// The deprecated top-level JWT directives map to an implicit issuer that is
	// only usable in compatibility mode. Enable it automatically so these
	// directives keep working instead of failing at provision, and warn to
	// steer users toward an issuer block. Requires a binary built with the
	// deprecated_claim tag to actually accept 0.x tokens.
	if m.ProtocolVersionCompatibility == 0 && m.hasLegacyVerifiers() {
		m.ProtocolVersionCompatibility = 8
		m.logger.Warn("Deprecated top-level JWT directives detected; enabling protocol_version_compatibility 8. Migrate them into an issuer block to run in modern mode.")
	}

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
	}

	if m.ResourceIdentifier != "" {
		opts = append(opts, mercure.WithResourceIdentifier(m.ResourceIdentifier))
	}

	if len(m.PublicURLs) > 0 {
		opts = append(opts, mercure.WithPublicURLs(m.PublicURLs))
	}

	if m.logger.Enabled(ctx, slog.LevelDebug) {
		opts = append(opts, mercure.WithDebug())
	}

	issuers, err := m.buildIssuers(ctx)
	if err != nil {
		return err
	}

	if len(issuers) > 0 {
		opts = append(opts, mercure.WithIssuers(issuers))
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

	if s := m.MaxRequestBodySize; s != nil {
		opts = append(opts, mercure.WithMaxRequestBodySize(*s))
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

	// Resolve the public origin from Caddy's request placeholders so it honors
	// the trusted_proxies configuration rather than raw forwarded headers. The
	// hub derives its OAuth resource identifier and RFC 9728 metadata URL from
	// it (a hub reachable through several public URLs needs no configuration),
	// and enforces the public_urls allowlist against this trusted origin.
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer) //nolint:forcetypeassert
	host, _ := repl.GetString("http.request.hostport")
	scheme, _ := repl.GetString("http.request.scheme")

	// Pass the origin out-of-band via the context, never by mutating r.URL: the
	// demo handler builds its rel="self" Link from r.URL.String(), which must
	// stay a relative path. Writing scheme/host onto r.URL would corrupt that
	// Link (and diverge under trusted_proxies).
	m.hub.ServeHTTP(w, r.WithContext(mercure.NewRequestOriginContext(r.Context(), scheme, host)))

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

			case "max_request_body_size":
				if !d.NextArg() {
					return d.ArgErr()
				}

				size, err := humanize.ParseBytes(d.Val())
				if err != nil || size > math.MaxInt64 {
					return d.Errf("invalid max_request_body_size %q", d.Val())
				}

				s := int64(size)
				m.MaxRequestBodySize = &s

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

			case "public_urls":
				m.PublicURLs = d.RemainingArgs()
				if len(m.PublicURLs) == 0 {
					return d.ArgErr()
				}

			case "resource_identifier":
				if !d.NextArg() {
					return d.ArgErr()
				}

				m.ResourceIdentifier = d.Val()

			case "issuer":
				ic, err := parseIssuerBlock(d)
				if err != nil {
					return err
				}

				m.Issuers = append(m.Issuers, ic)

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

// parseIssuerBlock parses an "issuer <identifier> { ... }" Caddyfile block.
func parseIssuerBlock(d *caddyfile.Dispenser) (IssuerConfig, error) {
	var ic IssuerConfig

	if !d.NextArg() {
		return ic, d.ArgErr() //nolint:wrapcheck
	}

	ic.Identifier = d.Val()

	for d.NextBlock(1) {
		switch d.Val() {
		case "authorization_server":
			ic.AuthorizationServer = true

		case "publisher":
			v, err := parseVerifierBlock(d)
			if err != nil {
				return ic, err
			}

			ic.Publisher = v

		case "subscriber":
			v, err := parseVerifierBlock(d)
			if err != nil {
				return ic, err
			}

			ic.Subscriber = v

		default:
			return ic, d.Errf("unknown issuer directive %q", d.Val()) //nolint:wrapcheck
		}
	}

	return ic, nil
}

// parseVerifierBlock parses a "publisher"/"subscriber" verifier subblock. The
// "jwt" and "jwks_uri" directives are mutually exclusive.
func parseVerifierBlock(d *caddyfile.Dispenser) (VerifierConfig, error) {
	var v VerifierConfig

	for d.NextBlock(2) {
		switch d.Val() {
		case "jwt":
			if v.JWKSURL != "" {
				return v, d.Err(`"jwt" and "jwks_uri" are mutually exclusive`) //nolint:wrapcheck
			}

			if !d.NextArg() {
				return v, d.ArgErr() //nolint:wrapcheck
			}

			v.JWT.Key = d.Val()
			if d.NextArg() {
				v.JWT.Alg = d.Val()
			}

		case "jwks_uri":
			if v.JWT.Key != "" {
				return v, d.Err(`"jwt" and "jwks_uri" are mutually exclusive`) //nolint:wrapcheck
			}

			if !d.NextArg() {
				return v, d.ArgErr() //nolint:wrapcheck
			}

			v.JWKSURL = d.Val()
			v.JWKSAlgorithms = d.RemainingArgs()

		default:
			return v, d.Errf("unknown verifier directive %q", d.Val()) //nolint:wrapcheck
		}
	}

	return v, nil
}

// normalizeJWT applies Caddy placeholder replacement to a static-key verifier
// and defaults its algorithm to HS256. It is a no-op when a JWK Set URL is used
// or no key is configured.
func normalizeJWT(repl *caddy.Replacer, c *JWTConfig, jwksURL string) {
	if jwksURL != "" {
		return
	}

	c.Key = repl.ReplaceKnown(c.Key, "")
	if c.Key == "" {
		return
	}

	c.Alg = repl.ReplaceKnown(c.Alg, "HS256")
	if c.Alg == "" {
		c.Alg = "HS256"
	}
}

func (m *Mercure) populateJWTConfig() error {
	repl := caddy.NewReplacer()

	normalizeJWT(repl, &m.PublisherJWT, m.PublisherJWKSURL)
	normalizeJWT(repl, &m.SubscriberJWT, m.SubscriberJWKSURL)

	hasPublisher := m.PublisherJWT.Key != "" || m.PublisherJWKSURL != ""
	hasSubscriber := m.SubscriberJWT.Key != "" || m.SubscriberJWKSURL != ""

	for i := range m.Issuers {
		iss := &m.Issuers[i]
		normalizeJWT(repl, &iss.Publisher.JWT, iss.Publisher.JWKSURL)
		normalizeJWT(repl, &iss.Subscriber.JWT, iss.Subscriber.JWKSURL)

		if iss.Publisher.isSet() {
			hasPublisher = true
		}

		if iss.Subscriber.isSet() {
			hasSubscriber = true
		}
	}

	if !hasPublisher && !AllowNoPublish {
		return errors.New("a JWT key or the URL of a JWK Set for publishers must be provided") //nolint:err113
	}

	if !hasSubscriber && !m.Anonymous {
		return errors.New("a JWT key or the URL of a JWK Set for subscribers must be provided") //nolint:err113
	}

	return nil
}

// buildVerifier turns a configured VerifierConfig into a mercure.Verifier. A
// JWK Set URL takes precedence over a static key. It is only called for a
// VerifierConfig that isSet reports as configured.
func (m *Mercure) buildVerifier(ctx context.Context, c VerifierConfig, role string) (mercure.Verifier, error) { //nolint:ireturn
	if c.JWKSURL != "" {
		k, err := newJWKSetKeyfunc(ctx, c.JWKSURL)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve %s JWK Set: %w", role, err)
		}

		return mercure.KeyFunc{Keyfunc: k.Keyfunc, Algorithms: c.JWKSAlgorithms}, nil
	}

	return mercure.Static{Key: []byte(c.JWT.Key), Algorithm: c.JWT.Alg}, nil
}

// buildIssuer builds one mercure.Issuer, skipping the verifier for an
// unconfigured role.
func (m *Mercure) buildIssuer(ctx context.Context, id string, authServer bool, pub, sub VerifierConfig) (mercure.Issuer, error) {
	issuer := mercure.Issuer{Identifier: id, AuthorizationServer: authServer}

	if pub.isSet() {
		v, err := m.buildVerifier(ctx, pub, "publisher")
		if err != nil {
			return issuer, err
		}

		issuer.Publisher = v
	}

	if sub.isSet() {
		v, err := m.buildVerifier(ctx, sub, "subscriber")
		if err != nil {
			return issuer, err
		}

		issuer.Subscriber = v
	}

	return issuer, nil
}

// legacyVerifiers returns the publisher and subscriber verifiers configured
// through the deprecated top-level directives (the single implicit issuer).
func (m *Mercure) legacyVerifiers() (VerifierConfig, VerifierConfig) {
	return VerifierConfig{JWT: m.PublisherJWT, JWKSURL: m.PublisherJWKSURL},
		VerifierConfig{JWT: m.SubscriberJWT, JWKSURL: m.SubscriberJWKSURL}
}

// hasLegacyVerifiers reports whether any deprecated top-level JWT directive is set.
func (m *Mercure) hasLegacyVerifiers() bool {
	pub, sub := m.legacyVerifiers()

	return pub.isSet() || sub.isSet()
}

// buildIssuers assembles the hub's issuer bindings from the explicit issuer
// blocks and the deprecated top-level directives (a single implicit issuer).
func (m *Mercure) buildIssuers(ctx context.Context) ([]mercure.Issuer, error) {
	issuers := make([]mercure.Issuer, 0, len(m.Issuers)+1)

	for _, ic := range m.Issuers {
		issuer, err := m.buildIssuer(ctx, ic.Identifier, ic.AuthorizationServer, ic.Publisher, ic.Subscriber)
		if err != nil {
			return nil, err
		}

		issuers = append(issuers, issuer)
	}

	legacyPub, legacySub := m.legacyVerifiers()

	if legacyPub.isSet() || legacySub.isSet() {
		issuer, err := m.buildIssuer(ctx, "", false, legacyPub, legacySub)
		if err != nil {
			return nil, err
		}

		issuers = append(issuers, issuer)
	}

	return issuers, nil
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
