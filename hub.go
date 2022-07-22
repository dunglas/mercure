// Package mercure helps implementing the Mercure protocol (https://mercure.rocks) in Go projects.
// It provides an implementation of a Mercure hub as a HTTP handler.
package mercure

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// ErrUnsupportedProtocolVersion is returned when the version passed is unsupported.
var ErrUnsupportedProtocolVersion = errors.New("compatibility mode only supports protocol version 7")

// Option instances allow to configure the library.
type Option func(h *opt) error

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
func WithLogger(logger Logger) Option {
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

// WithPublisherJWT sets the JWT key and the signing algorithm to use for publishers.
func WithPublisherJWT(key []byte, alg string) Option {
	return func(o *opt) error {
		sm := jwt.GetSigningMethod(alg)
		switch sm.(type) {
		case *jwt.SigningMethodHMAC:
		case *jwt.SigningMethodRSA:
		default:
			return ErrUnexpectedSigningMethod
		}

		o.publisherJWT = &jwtConfig{key, sm}

		return nil
	}
}

// WithSubscriberJWT sets the JWT key and the signing algorithm to use for subscribers.
func WithSubscriberJWT(key []byte, alg string) Option {
	return func(o *opt) error {
		sm := jwt.GetSigningMethod(alg)
		switch sm.(type) {
		case *jwt.SigningMethodHMAC:
		case *jwt.SigningMethodRSA:
		default:
			return ErrUnexpectedSigningMethod
		}

		o.subscriberJWT = &jwtConfig{key, sm}

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

// WithPublishOrigins sets the origins allowed to publish updates.
func WithPublishOrigins(origins []string) Option {
	return func(o *opt) error {
		o.publishOrigins = origins

		return nil
	}
}

// WithCORSOrigins sets the allowed CORS origins.
func WithCORSOrigins(origins []string) Option {
	return func(o *opt) error {
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

// WithProtocolVersionCompatibility sets the version of the Mercure protocol to be backward compatible with (only version 7 is supported).
func WithProtocolVersionCompatibility(protocolVersionCompatibility int) Option {
	return func(o *opt) error {
		if protocolVersionCompatibility != 7 {
			return ErrUnsupportedProtocolVersion
		}

		o.protocolVersionCompatibility = protocolVersionCompatibility

		return nil
	}
}

type jwtConfig struct {
	key           []byte
	signingMethod jwt.SigningMethod
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
	logger                       Logger
	writeTimeout                 time.Duration
	dispatchTimeout              time.Duration
	heartbeat                    time.Duration
	publisherJWT                 *jwtConfig
	subscriberJWT                *jwtConfig
	metrics                      Metrics
	allowedHosts                 []string
	publishOrigins               []string
	corsOrigins                  []string
	cookieName                   string
	protocolVersionCompatibility int
}

func (o *opt) isBackwardCompatiblyEnabledWith(version int) bool {
	return o.protocolVersionCompatibility != 0 && version >= o.protocolVersionCompatibility
}

// Hub stores channels with clients currently subscribed and allows to dispatch updates.
type Hub struct {
	*opt
	handler http.Handler

	// Deprecated: use the Caddy server module or the standalone library instead.
	config        *viper.Viper
	server        *http.Server
	metricsServer *http.Server
}

// NewHub creates a new Hub instance.
func NewHub(options ...Option) (*Hub, error) {
	opt := &opt{writeTimeout: 600 * time.Second}

	for _, o := range options {
		if err := o(opt); err != nil {
			return nil, err
		}
	}

	if opt.logger == nil {
		var (
			l   Logger
			err error
		)
		if opt.debug {
			l, err = zap.NewDevelopment()
		} else {
			l, err = zap.NewProduction()
		}

		if err != nil {
			return nil, fmt.Errorf("error when creating logger: %w", err)
		}

		opt.logger = l
	}

	if opt.transport == nil {
		t, _ := NewLocalTransport(nil, nil, nil)
		opt.transport = t
	}

	if opt.topicSelectorStore == nil {
		tss, err := NewTopicSelectorStoreRistretto(TopicSelectorStoreRistrettoDefaultCacheNumCounters, TopicSelectorStoreRistrettoCacheMaxCost)
		if err != nil {
			return nil, err
		}

		opt.topicSelectorStore = tss
	}

	if opt.metrics == nil {
		opt.metrics = NopMetrics{}
	}

	if opt.cookieName == "" {
		opt.cookieName = defaultCookieName
	}

	h := &Hub{opt: opt}
	h.initHandler()

	return h, nil
}

// Stop stops the hub.
func (h *Hub) Stop() error {
	if err := h.transport.Close(); err != nil {
		return fmt.Errorf("transport error: %w", err)
	}

	return nil
}
