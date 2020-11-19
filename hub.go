package mercure

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const HS256 = "HS256"

// Option instances allow to configure the library.
type Option func(h *opt)

// WithAnonymous allows subscribers with no valid JWT to connect.
func WithAnonymous() Option {
	return func(o *opt) {
		o.anonymous = true
	}
}

// WithDebug enables the debug mode.
func WithDebug() Option {
	return func(o *opt) {
		o.debug = true
	}
}

// WithDemo enables the demo mode.
func WithDemo() Option {
	return func(o *opt) {
		o.demo = true
	}
}

// WithMetrics enables collection of Prometheus metrics.
func WithMetrics() Option {
	return func(o *opt) {
		o.metrics = true
	}
}

// WithSubscriptions allows to dispatch updates when subscriptions are created or terminated.
func WithSubscriptions() Option {
	return func(o *opt) {
		o.subscriptions = true
	}
}

// WithLogger sets the logger to use.
func WithLogger(logger Logger) Option {
	return func(o *opt) {
		o.logger = logger
	}
}

// WithWriteTimeout sets maximum duration before closing the connection, defaults to 600s, set to 0 to disable.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(o *opt) {
		o.writeTimeout = timeout
	}
}

// WithDispatchTimeout sets the maximum dispatch duration of an update.
func WithDispatchTimeout(timeout time.Duration) Option {
	return func(o *opt) {
		o.dispatchTimeout = timeout
	}
}

// WithHeartbeat enables heartbeat.
func WithHeartbeat(interval time.Duration) Option {
	return func(o *opt) {
		o.heartbeat = interval
	}
}

// WithPublisherJWTConfig sets the JWT key and algorithm to use.
func WithPublisherJWTConfig(key []byte, signingMethod jwt.SigningMethod) Option {
	return func(o *opt) {
		o.publisherJWTConfig = &jwtConfig{key, signingMethod}
	}
}

// WithSubscriberJWTConfig sets the JWT key and algorithm to use.
func WithSubscriberJWTConfig(key []byte, signingMethod jwt.SigningMethod) Option {
	return func(o *opt) {
		o.subscriberJWTConfig = &jwtConfig{key, signingMethod}
	}
}

// WithPublishOrigins sets the origins allowed to publish updates.
func WithPublishOrigins(origins []string) Option {
	return func(o *opt) {
		o.publishOrigins = origins
	}
}

// WithTransportURL sets the transport to use by parsing the provided URL.
func WithTransportURL(tu string) Option {
	u, err := url.Parse(tu)
	if err != nil {
		log.Panic(fmt.Errorf("transport_url: %w", err))
	}

	return func(o *opt) {
		o.transportURL = u
	}
}

type jwtConfig struct {
	key           []byte
	signingMethod jwt.SigningMethod
}

type opt struct {
	anonymous           bool
	debug               bool
	demo                bool
	metrics             bool
	subscriptions       bool
	logger              Logger
	writeTimeout        time.Duration
	dispatchTimeout     time.Duration
	heartbeat           time.Duration
	publisherJWTConfig  *jwtConfig
	subscriberJWTConfig *jwtConfig
	publishOrigins      []string
	transportURL        *url.URL
}

// Hub stores channels with clients currently subscribed and allows to dispatch updates.
type Hub struct {
	*opt
	transport          Transport
	metrics            *Metrics
	topicSelectorStore *TopicSelectorStore

	// Deprecated: use the Caddy server module or the standalone library instead.
	config        *viper.Viper
	server        *http.Server
	metricsServer *http.Server
}

func New(options ...Option) *Hub {
	opt := &opt{
		writeTimeout: 600 * time.Second,
	}

	for _, o := range options {
		o(opt)
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
			log.Panic(err)
		}

		opt.logger = l
	}

	if opt.transportURL == nil {
		opt.transportURL = &url.URL{Scheme: "local"}
	}

	t, err := newTransport(opt.transportURL, opt.logger)
	if err != nil {
		log.Panic(err)
	}

	h := &Hub{
		opt:                opt,
		transport:          t,
		topicSelectorStore: NewTopicSelectorStore(),
	}

	if opt.metrics {
		h.metrics = NewMetrics()
	}

	return h
}

// Stop stops disconnect all connected clients.
func (h *Hub) Stop() error {
	return h.transport.Close()
}
