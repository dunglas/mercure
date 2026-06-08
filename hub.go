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

		return err
	}
}

// WithSubscriberJWT sets the JWT key and the signing algorithm to use for subscribers.
func WithSubscriberJWT(key []byte, alg string) Option {
	return func(o *opt) error {
		keyfunc, err := createJWTKeyfunc(key, alg)
		o.subscriberJWTKeyFunc = keyfunc

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

// WithCookieName sets the name of the authorization cookie (defaults to "mercureAuthorization").
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
func WithAuthorizationServers(authorizationServers []string) Option {
	return func(o *opt) error {
		o.authorizationServers = authorizationServers

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

	opt.topicSelectorStore.setBaseURL(opt.publicURL)

	if opt.resourceIdentifier == "" {
		opt.resourceIdentifier = opt.publicURL
	}

	// In modern mode, RFC 9068 requires checking the token audience against the
	// hub's resource identifier; refuse to start a token-validating hub that
	// cannot perform that check.
	if opt.resourceIdentifier == "" && opt.protocolVersionCompatibility == 0 &&
		(opt.publisherJWTKeyFunc != nil || opt.subscriberJWTKeyFunc != nil) {
		return nil, ErrMissingResourceIdentifier
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
