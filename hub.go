// Package mercure helps implement the Mercure protocol (https://mercure.rocks) in Go projects.
// It provides an implementation of a Mercure hub as an HTTP handler.
package mercure

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	DefaultWriteTimeout    = 600 * time.Second
	DefaultDispatchTimeout = 5 * time.Second
	DefaultHeartbeat       = 40 * time.Second
)

// ErrUnsupportedProtocolVersion is returned when the version passed is unsupported.
var ErrUnsupportedProtocolVersion = errors.New("compatibility mode only supports protocol versions 7 and 8")

// ErrMissingResourceIdentifier is returned when the hub validates access
// tokens in modern mode but no resource identifier (nor public URL) is set:
// RFC 9068 requires the token audience to be checked against it.
var ErrMissingResourceIdentifier = errors.New("a resource identifier (or public URL) is required when JWT authentication is enabled; set WithResourceIdentifier or WithPublicURL, or enable compatibility mode")

// ErrInvalidResourceIdentifier is returned when the configured resource
// identifier is not an RFC 9728 protected resource identifier: an absolute
// URL without a fragment component.
var ErrInvalidResourceIdentifier = errors.New("the resource identifier must be an absolute URL without a fragment (RFC 9728)")

// schemeHTTPS is the URL scheme required by RFC 9728 resource identifiers.
const schemeHTTPS = "https"

// defaultJWTAlgorithms is the signature-algorithm allowlist applied in modern
// mode when a JWT key function is configured without an explicit list (the
// JWKS path). It contains only asymmetric algorithms: allowing an HMAC
// algorithm next to public keys would enable algorithm-confusion attacks.
//
//nolint:gochecknoglobals
var defaultJWTAlgorithms = []string{"EdDSA", "ES256", "ES384", "ES512", "RS256", "RS384", "RS512", "PS256", "PS384", "PS512"}

// Option instances allow to configure the library.
type Option func(o *opt) error

// WithAnonymous allows subscribers with no valid JWT.
func WithAnonymous() Option {
	return func(o *opt) error {
		o.anonymous = true

		return nil
	}
}

// WithDebug enables the debug mode.
func WithDebug() Option {
	return func(o *opt) error {
		o.debug = true

		return nil
	}
}

func WithUI() Option {
	return func(o *opt) error {
		o.ui = true

		return nil
	}
}

// WithDemo enables the demo.
func WithDemo() Option {
	return func(o *opt) error {
		o.demo = true
		o.ui = true

		return nil
	}
}

// WithMetrics enables collection of Prometheus metrics.
func WithMetrics(m Metrics) Option {
	return func(o *opt) error {
		o.metrics = m

		return nil
	}
}

// WithSubscriptions allows to dispatch updates when subscriptions are created or terminated.
func WithSubscriptions() Option {
	return func(o *opt) error {
		o.subscriptions = true

		return nil
	}
}

// WithLogger sets the logger to use.
func WithLogger(logger *slog.Logger) Option {
	return func(o *opt) error {
		o.logger = logger

		return nil
	}
}

// WithWriteTimeout sets maximum duration before closing the connection, defaults to 600s, set to 0 to disable.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(o *opt) error {
		o.writeTimeout = timeout

		return nil
	}
}

// WithDispatchTimeout sets maximum dispatch duration of an update.
func WithDispatchTimeout(timeout time.Duration) Option {
	return func(o *opt) error {
		o.dispatchTimeout = timeout

		return nil
	}
}

// WithHeartbeat sets the frequency of the heartbeat, disabled by default.
func WithHeartbeat(interval time.Duration) Option {
	return func(o *opt) error {
		o.heartbeat = interval

		return nil
	}
}

// WithPublisherJWTKeyFunc sets the function to use to parse and verify the publisher JWT.
func WithPublisherJWTKeyFunc(keyfunc jwt.Keyfunc) Option {
	return func(o *opt) error {
		o.publisherJWTKeyFunc = keyfunc

		return nil
	}
}

// WithSubscriberJWTKeyFunc sets the function to use to parse and verify the subscriber JWT.
func WithSubscriberJWTKeyFunc(keyfunc jwt.Keyfunc) Option {
	return func(o *opt) error {
		o.subscriberJWTKeyFunc = keyfunc

		return nil
	}
}

// WithPublisherJWT sets the JWT key and the signing algorithm to use for publishers.
func WithPublisherJWT(key []byte, alg string) Option {
	return func(o *opt) error {
		keyfunc, err := createJWTKeyfunc(key, alg)
		o.publisherJWTKeyFunc = keyfunc
		o.publisherJWTAlgorithms = []string{alg}

		return err
	}
}

// WithSubscriberJWT sets the JWT key and the signing algorithm to use for subscribers.
func WithSubscriberJWT(key []byte, alg string) Option {
	return func(o *opt) error {
		keyfunc, err := createJWTKeyfunc(key, alg)
		o.subscriberJWTKeyFunc = keyfunc
		o.subscriberJWTAlgorithms = []string{alg}

		return err
	}
}

// WithAllowedHosts sets the allowed hosts.
func WithAllowedHosts(hosts []string) Option {
	return func(o *opt) error {
		o.allowedHosts = hosts

		return nil
	}
}

func validateOrigins(origins []string) error {
	for _, origin := range origins {
		switch origin {
		case "*", "null": //nolint:goconst
			continue
		}

		u, err := url.Parse(origin)
		if err != nil ||
			!u.IsAbs() ||
			u.Opaque != "" ||
			u.User != nil ||
			u.Path != "" ||
			u.RawQuery != "" ||
			u.Fragment != "" {
			return fmt.Errorf(`invalid origin, must be a URL having only a scheme, a host and optionally a port, "*" or "null": %w`, err)
		}
	}

	return nil
}

// WithPublishOrigins sets the origins allowed to publish updates.
func WithPublishOrigins(origins []string) Option {
	return func(o *opt) error {
		if err := validateOrigins(origins); err != nil {
			return err
		}

		// wildcard support has been adapted from https://github.com/rs/cors/blob/1084d89a16921942356d1c831fbe523426cf836e/cors.go#L171
		// Copyright (c) 2014 Olivier Poitrey <rs@dailymotion.com>
		// MIT licensed.
		for _, origin := range origins {
			// Note: for origins matching, the spec requires a case-sensitive matching.
			// As it may error-prone, we chose to ignore the spec here.
			origin = strings.ToLower(origin)
			if origin == "*" {
				// If "*" is present in the list, turn the whole list into a match all
				o.publishOriginsAll = true
				o.publishOrigins = nil
				o.publishWOrigins = nil

				break
			} else if prefix, suffix, found := strings.Cut(origin, "*"); found {
				// Split the origin in two: start and end string without the *
				w := wildcard{prefix, suffix}
				o.publishWOrigins = append(o.publishWOrigins, w)
			} else {
				o.publishOrigins = append(o.publishOrigins, origin)
			}
		}

		return nil
	}
}

// WithCORSOrigins sets the allowed CORS origins.
func WithCORSOrigins(origins []string) Option {
	return func(o *opt) error {
		if err := validateOrigins(origins); err != nil {
			return err
		}

		o.corsOrigins = origins

		return nil
	}
}

// WithTransport sets the transport to use.
func WithTransport(t Transport) Option {
	return func(o *opt) error {
		o.transport = t

		return nil
	}
}

// WithTopicSelectorStore sets the TopicSelectorStore instance to use.
func WithTopicSelectorStore(tss *TopicSelectorStore) Option {
	return func(o *opt) error {
		o.topicSelectorStore = tss

		return nil
	}
}

// WithCookieName sets the name of the authorization cookie (defaults to "mercure_access_token").
func WithCookieName(cookieName string) Option {
	return func(o *opt) error {
		o.cookieName = cookieName

		return nil
	}
}

// WithProtocolVersionCompatibility sets the version of the Mercure protocol
// to be backward compatible with (versions 7 and 8 are supported). The v8
// behaviors (URI Template selectors in the `topic` parameter, bare-string
// JWT claims, alternate topics, v8 subscription routes) additionally require
// a hub binary built with the deprecated_topic tag.
func WithProtocolVersionCompatibility(protocolVersionCompatibility int) Option {
	return func(o *opt) error {
		switch protocolVersionCompatibility {
		case 7, 8:
			o.protocolVersionCompatibility = protocolVersionCompatibility

			return nil
		default:
			return ErrUnsupportedProtocolVersion
		}
	}
}

// WithPublicURL sets the URL at which subscribers reach the hub. This is the
// full hub URL, including the "/.well-known/mercure" path (e.g.
// "https://example.com/.well-known/mercure"), not just the origin.
//
// It is used as the base URL when matching relative URL patterns and topics,
// per the protocol's "the hub MUST use the hub's URL as the base URL" rule
// (without it, only relative ↔ relative and absolute ↔ absolute matches work);
// as the default resource identifier (RFC 9068 aud) when WithResourceIdentifier
// is unset; and to derive the protected resource metadata URL. Omitting the
// "/.well-known/mercure" suffix degrades that derivation to a request-based
// fallback.
func WithPublicURL(publicURL string) Option {
	return func(o *opt) error {
		o.publicURL = publicURL

		return nil
	}
}

// WithResourceIdentifier sets the hub's OAuth 2.0 resource identifier (RFC 9068
// `aud` value, advertised through RFC 9728 protected resource metadata). It
// defaults to the public URL when unset.
func WithResourceIdentifier(resourceIdentifier string) Option {
	return func(o *opt) error {
		o.resourceIdentifier = resourceIdentifier

		return nil
	}
}

// WithAuthorizationServers sets the OAuth 2.0 authorization server issuer
// identifiers advertised in the hub's protected resource metadata (RFC 9728).
//
// When set, the token issuer (iss) check applies to both publisher and
// subscriber tokens: a token whose iss is not one of these identifiers is
// rejected. A self-issued token (for example a publisher signing with a key
// shared out of band) must therefore carry a matching iss, or this option must
// be left unset.
func WithAuthorizationServers(authorizationServers []string) Option {
	return func(o *opt) error {
		o.authorizationServers = authorizationServers

		return nil
	}
}

// WithPublisherJWTAlgorithms pins the JWS algorithms accepted for publisher
// access tokens. WithPublisherJWT already pins its single algorithm; this
// option is for the WithPublisherJWTKeyFunc / JWKS path, where the algorithm
// would otherwise be taken from the token header. Setting it makes the parser
// reject any token whose alg is not in the list (RFC 8725).
func WithPublisherJWTAlgorithms(algorithms []string) Option {
	return func(o *opt) error {
		o.publisherJWTAlgorithms = algorithms

		return nil
	}
}

// WithSubscriberJWTAlgorithms pins the JWS algorithms accepted for subscriber
// access tokens. See WithPublisherJWTAlgorithms.
func WithSubscriberJWTAlgorithms(algorithms []string) Option {
	return func(o *opt) error {
		o.subscriberJWTAlgorithms = algorithms

		return nil
	}
}

// opt contains the available options.
//
// If you change this, also update the Caddy module and the documentation.
type opt struct {
	transport                    Transport
	topicSelectorStore           *TopicSelectorStore
	anonymous                    bool
	debug                        bool
	subscriptions                bool
	ui                           bool
	demo                         bool
	logger                       *slog.Logger
	writeTimeout                 time.Duration
	dispatchTimeout              time.Duration
	heartbeat                    time.Duration
	publisherJWTKeyFunc          jwt.Keyfunc
	subscriberJWTKeyFunc         jwt.Keyfunc
	publisherJWTAlgorithms       []string
	subscriberJWTAlgorithms      []string
	metrics                      Metrics
	allowedHosts                 []string
	publishOriginsAll            bool
	publishOrigins               []string
	publishWOrigins              []wildcard
	corsOrigins                  []string
	cookieName                   string
	protocolVersionCompatibility int
	publicURL                    string
	resourceIdentifier           string
	authorizationServers         []string
}

// configureIdentifiers wires the public URL, the URL Pattern base, and the
// resource identifier defaults, then applies the modern-mode rules.
func (o *opt) configureIdentifiers() error {
	// When only the resource identifier is configured and it is a full hub
	// URL, it is the hub URL: use it as the URL Pattern base so relative
	// patterns and topics resolve per the protocol without requiring the
	// public URL to be configured twice.
	if o.publicURL == "" && strings.HasSuffix(o.resourceIdentifier, defaultHubURL) {
		o.publicURL = o.resourceIdentifier
	}

	if err := o.topicSelectorStore.setBaseURL(o.publicURL); err != nil {
		return err
	}

	if o.resourceIdentifier == "" {
		o.resourceIdentifier = o.publicURL
	}

	// In modern mode, RFC 9068 requires checking the token audience against
	// the hub's resource identifier; refuse to start a token-validating hub
	// that cannot perform that check.
	if o.resourceIdentifier == "" && o.protocolVersionCompatibility == 0 &&
		(o.publisherJWTKeyFunc != nil || o.subscriberJWTKeyFunc != nil) {
		return ErrMissingResourceIdentifier
	}

	return o.applyModernDefaults()
}

// applyModernDefaults enforces the modern-mode (non-compatibility) config
// rules: an RFC 9728-shaped resource identifier and an explicit JWS
// algorithm allowlist for every configured key function.
func (o *opt) applyModernDefaults() error {
	if o.protocolVersionCompatibility != 0 {
		return nil
	}

	// The resource identifier is published as the RFC 9728 "resource" member
	// and checked against the token audience; strict clients ignore metadata
	// whose identifier is not a URL without a fragment.
	if o.resourceIdentifier != "" {
		u, err := url.Parse(o.resourceIdentifier)
		if err != nil || !u.IsAbs() || u.Host == "" || u.Fragment != "" {
			return ErrInvalidResourceIdentifier
		}

		if u.Scheme != schemeHTTPS {
			o.logger.Warn(`The resource identifier does not use the "https" scheme; strict RFC 9728 clients will ignore the hub's protected resource metadata.`)
		}
	}

	// The protocol requires an explicit signature-algorithm allowlist that is
	// never derived from the token. Key functions configured without one (the
	// JWKS path) get the documented default: asymmetric algorithms only, so a
	// public JWK can never be reinterpreted as an HMAC secret.
	if o.publisherJWTKeyFunc != nil && len(o.publisherJWTAlgorithms) == 0 {
		o.publisherJWTAlgorithms = defaultJWTAlgorithms
	}

	if o.subscriberJWTKeyFunc != nil && len(o.subscriberJWTAlgorithms) == 0 {
		o.subscriberJWTAlgorithms = defaultJWTAlgorithms
	}

	return nil
}

func (o *opt) isBackwardCompatiblyEnabledWith(version int) bool {
	return o.protocolVersionCompatibility != 0 && version >= o.protocolVersionCompatibility
}

// Hub stores channels with clients currently subscribed and allows to dispatch updates.
type Hub struct {
	deprecatedHub
	*opt

	handler http.Handler
	ctx     context.Context //nolint:containedctx
}

// NewHub creates a new Hub instance.
func NewHub(ctx context.Context, options ...Option) (*Hub, error) {
	opt := &opt{
		writeTimeout:    DefaultWriteTimeout,
		dispatchTimeout: DefaultDispatchTimeout,
		heartbeat:       DefaultHeartbeat,
	}

	for _, o := range options {
		if err := o(opt); err != nil {
			return nil, err
		}
	}

	if opt.logger == nil {
		opt.logger = slog.New(mercureHandler{slog.Default().Handler()})
	}

	if opt.topicSelectorStore == nil {
		tss, err := NewTopicSelectorStore(DefaultTopicSelectorStoreCacheSize)
		if err != nil {
			return nil, err
		}

		opt.topicSelectorStore = tss
	}

	if err := opt.configureIdentifiers(); err != nil {
		return nil, err
	}

	if opt.transport == nil {
		opt.transport = NewLocalTransport(NewSubscriberList(DefaultSubscriberListCacheSize))
	}

	if ttss, ok := opt.transport.(TransportTopicSelectorStore); ok {
		ttss.SetTopicSelectorStore(opt.topicSelectorStore)
	}

	if opt.metrics == nil {
		opt.metrics = NopMetrics{}
	}

	if opt.cookieName == "" {
		opt.cookieName = defaultCookieName
	}

	h := &Hub{opt: opt, ctx: ctx}
	h.initHandler()

	return h, nil
}

// Stop stops the hub.
func (h *Hub) Stop(ctx context.Context) error {
	if err := h.transport.Close(ctx); err != nil {
		return fmt.Errorf("transport error: %w", err)
	}

	return nil
}
