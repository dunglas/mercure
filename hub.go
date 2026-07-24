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

	// DefaultMaxRequestBodySize bounds the publish and QUERY subscribe request
	// bodies; larger requests are rejected with a 413 as the protocol requires.
	DefaultMaxRequestBodySize int64 = 1 << 20 // 1 MiB
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

// ErrMissingIssuerIdentifier is returned when an issuer is configured in modern
// mode with an empty identifier: the token iss claim is matched against it, so
// it must be set.
var ErrMissingIssuerIdentifier = errors.New("an issuer identifier is required in modern mode")

// ErrDuplicateIssuer is returned when the same issuer identifier is configured
// more than once.
var ErrDuplicateIssuer = errors.New("duplicate issuer identifier")

// ErrIssuerMissingKey is returned when an issuer has no verifier for either
// role, so no token could ever be verified for it.
var ErrIssuerMissingKey = errors.New("an issuer must configure a publisher or subscriber verifier")

// ErrMissingAlgorithm is returned when a Static verifier is configured without
// a signing algorithm.
var ErrMissingAlgorithm = errors.New("a Static verifier requires a signing algorithm")

// schemeHTTPS is the URL scheme required by RFC 9728 resource identifiers.
const schemeHTTPS = "https"

// defaultJWTAlgorithms is the signature-algorithm allowlist applied, in every
// mode, when a JWT key function is configured without an explicit list (the
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

// WithHeartbeat sets the frequency of the SSE keep-alive comments, defaults
// to 40s, set to 0 to disable.
func WithHeartbeat(interval time.Duration) Option {
	return func(o *opt) error {
		o.heartbeat = interval

		return nil
	}
}

// WithMaxRequestBodySize bounds the size, in bytes, of publish and QUERY
// subscribe request bodies; larger requests are rejected with a 413 status
// code. Defaults to DefaultMaxRequestBodySize, set to 0 to disable the
// in-hub limit (for example, when a reverse proxy already enforces one).
func WithMaxRequestBodySize(size int64) Option {
	return func(o *opt) error {
		o.maxRequestBodySize = size

		return nil
	}
}

// WithIssuers configures access-token verification, binding each trusted issuer
// (RFC 9068 §4) to its own verification material. It replaces the former
// per-role options: a token is verified only with the key(s) associated with
// its iss claim, so key material is never pooled across issuers.
//
// Each Issuer provides a Publisher and/or Subscriber Verifier (a nil Verifier
// means that role is not accepted for the issuer). Setting AuthorizationServer
// advertises the issuer in the hub's RFC 9728 protected resource metadata; a
// self-issued issuer (a key shared out of band) leaves it false.
func WithIssuers(issuers []Issuer) Option {
	return func(o *opt) error {
		if o.issuers == nil {
			o.issuers = make(map[string]issuerVerifier, len(issuers))
		}

		for _, iss := range issuers {
			if _, ok := o.issuers[iss.Identifier]; ok {
				return fmt.Errorf("%w: %q", ErrDuplicateIssuer, iss.Identifier)
			}

			var iv issuerVerifier

			if iss.Publisher != nil {
				kf, algs, err := iss.Publisher.buildKeyfunc()
				if err != nil {
					return err
				}

				iv.publisher = roleVerifier{keyfunc: kf, algorithms: algs}
				o.publisherConfigured = true
			}

			if iss.Subscriber != nil {
				kf, algs, err := iss.Subscriber.buildKeyfunc()
				if err != nil {
					return err
				}

				iv.subscriber = roleVerifier{keyfunc: kf, algorithms: algs}
				o.subscriberConfigured = true
			}

			if iv.publisher.keyfunc == nil && iv.subscriber.keyfunc == nil {
				return fmt.Errorf("%w: %q", ErrIssuerMissingKey, iss.Identifier)
			}

			o.issuers[iss.Identifier] = iv

			if iss.AuthorizationServer {
				o.authorizationServers = append(o.authorizationServers, iss.Identifier)
			}
		}

		return nil
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

// WithTopicMatcherStore sets the TopicMatcherStore instance to use.
func WithTopicMatcherStore(tms *TopicMatcherStore) Option {
	return func(o *opt) error {
		o.topicMatcherStore = tms

		return nil
	}
}

// WithCookieName sets the name of the authorization cookie (defaults to
// "__Secure-mercure_access_token"). The default "__Secure-" prefix makes user
// agents refuse the cookie over insecure transport; plain-HTTP deployments
// (local development) must configure a prefix-less name.
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

// opt contains the available options.
//
// If you change this, also update the Caddy module and the documentation.
type opt struct {
	transport                    Transport
	topicMatcherStore            *TopicMatcherStore
	anonymous                    bool
	debug                        bool
	subscriptions                bool
	ui                           bool
	demo                         bool
	logger                       *slog.Logger
	writeTimeout                 time.Duration
	dispatchTimeout              time.Duration
	heartbeat                    time.Duration
	maxRequestBodySize           int64
	issuers                      map[string]issuerVerifier
	publisherConfigured          bool
	subscriberConfigured         bool
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
	resourceMetadataURL          string
	authorizationServers         []string
}

// roleVerifier holds the verification material for one role of one issuer.
type roleVerifier struct {
	keyfunc    jwt.Keyfunc
	algorithms []string
}

// issuerVerifier binds an issuer to its per-role verification material. A role
// with a nil keyfunc is not accepted for the issuer.
type issuerVerifier struct {
	publisher  roleVerifier
	subscriber roleVerifier
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

	if err := o.topicMatcherStore.setBaseURL(o.publicURL); err != nil {
		return err
	}

	if o.resourceIdentifier == "" {
		o.resourceIdentifier = o.publicURL
	}

	// Build the RFC 9728 metadata URL once, here, rather than on every
	// unauthenticated request that emits a Bearer challenge.
	o.resourceMetadataURL = buildResourceMetadataURL(o.publicURL, o.resourceIdentifier)

	// In modern mode, RFC 9068 requires checking the token audience against
	// the hub's resource identifier; refuse to start a token-validating hub
	// that cannot perform that check.
	if o.resourceIdentifier == "" && o.protocolVersionCompatibility == 0 &&
		(o.publisherConfigured || o.subscriberConfigured) {
		return ErrMissingResourceIdentifier
	}

	// In modern mode the token iss claim must exactly match a trusted issuer,
	// so every configured issuer needs a non-empty identifier to match against.
	// (Configuring a verifier always creates an issuer, so no separate
	// "missing issuer" case exists.)
	if o.protocolVersionCompatibility == 0 && (o.publisherConfigured || o.subscriberConfigured) {
		if _, ok := o.issuers[""]; ok {
			return ErrMissingIssuerIdentifier
		}
	}

	return o.applyModernDefaults()
}

// applyModernDefaults enforces the modern-mode invariant of an RFC 9728-shaped
// resource identifier. The JWS algorithm allowlist is pinned per issuer when
// the verifiers are built (see WithIssuers and buildKeyfunc).
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

	return nil
}

func (o *opt) isBackwardCompatiblyEnabledWith(version int) bool {
	return o.protocolVersionCompatibility != 0 && version >= o.protocolVersionCompatibility
}

// Hub stores channels with clients currently subscribed and allows to dispatch updates.
type Hub struct {
	*opt

	handler http.Handler
	ctx     context.Context //nolint:containedctx
}

// NewHub creates a new Hub instance.
func NewHub(ctx context.Context, options ...Option) (*Hub, error) {
	opt := &opt{
		writeTimeout:       DefaultWriteTimeout,
		dispatchTimeout:    DefaultDispatchTimeout,
		heartbeat:          DefaultHeartbeat,
		maxRequestBodySize: DefaultMaxRequestBodySize,
	}

	for _, o := range options {
		if err := o(opt); err != nil {
			return nil, err
		}
	}

	if opt.logger == nil {
		opt.logger = slog.New(mercureHandler{slog.Default().Handler()})
	}

	if opt.topicMatcherStore == nil {
		tms, err := NewTopicMatcherStore(DefaultTopicMatcherStoreCacheSize)
		if err != nil {
			return nil, err
		}

		opt.topicMatcherStore = tms
	}

	if err := opt.configureIdentifiers(); err != nil {
		return nil, err
	}

	if opt.transport == nil {
		opt.transport = NewLocalTransport(NewSubscriberList(DefaultSubscriberListCacheSize))
	}

	if ttss, ok := opt.transport.(TransportTopicMatcherStore); ok {
		ttss.SetTopicMatcherStore(opt.topicMatcherStore)
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

// limitRequestBody bounds the request body per WithMaxRequestBodySize; the
// protocol requires rejecting larger requests with a 413 status code, which
// handlers detect through the *http.MaxBytesError read failure.
func (h *Hub) limitRequestBody(w http.ResponseWriter, r *http.Request) {
	if h.maxRequestBodySize > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, h.maxRequestBodySize)
	}
}
