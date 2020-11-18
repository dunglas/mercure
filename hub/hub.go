package hub

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
type Option func(h *Hub)

// WithAnonymous allows subscribers with no valid JWT to connect.
func WithAnonymous() Option {
	return func(h *Hub) {
		h.anonymous = true
	}
}

// WithDebug enables the debug mode.
func WithDebug() Option {
	return func(o *Hub) {
		o.debug = true
	}
}

// WithDemo enables the demo mode.
func WithDemo() Option {
	return func(h *Hub) {
		h.demo = true
	}
}

// WithMetrics enables collection of Prometheus metrics.
func WithMetrics() Option {
	return func(h *Hub) {
		h.metrics = NewMetrics()
	}
}

// WithSubscriptions allows to dispatch updates when subscriptions are created or terminated.
func WithSubscriptions() Option {
	return func(h *Hub) {
		h.subscriptions = true
	}
}

// WithLogger sets the logger to use.
func WithLogger(logger Logger) Option {
	return func(h *Hub) {
		h.logger = logger
	}
}

// WithWriteTimeout sets maximum duration before closing the connection, defaults to 600s, set to 0 to disable.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(h *Hub) {
		h.writeTimeout = timeout
	}
}

// WithDispatchTimeout sets the maximum dispatch duration of an update.
func WithDispatchTimeout(timeout time.Duration) Option {
	return func(h *Hub) {
		h.dispatchTimeout = timeout
	}
}

// WithHeartbeat enables heartbeat.
func WithHeartbeat(interval time.Duration) Option {
	return func(h *Hub) {
		h.heartbeat = interval
	}
}

// WithPublisherJWTSigningMethod sets the JWT algorithm to use.
func WithPublisherJWTSigningMethod(signingMethod jwt.SigningMethod) Option {
	return func(h *Hub) {
		h.publisherJWTConfig.signingMethod = signingMethod
	}
}

// WithSubscriberJWTConfig sets the JWT key and algorithm to use.
func WithSubscriberJWTConfig(key []byte, signingMethod jwt.SigningMethod) Option {
	return func(h *Hub) {
		h.subscriberJWTConfig.key = key
		h.subscriberJWTConfig.signingMethod = signingMethod
	}
}

// WithPublishOrigins sets the origins allowed to publish updates.
func WithPublishOrigins(origins []string) Option {
	return func(h *Hub) {
		h.publishOrigins = origins
	}
}

type jwtConfig struct {
	key           []byte
	signingMethod jwt.SigningMethod
}

// Hub stores channels with clients currently subscribed and allows to dispatch updates.
type Hub struct {
	anonymous           bool
	debug               bool
	demo                bool
	subscriptions       bool
	logger              Logger
	writeTimeout        time.Duration
	dispatchTimeout     time.Duration
	heartbeat           time.Duration
	publisherJWTConfig  jwtConfig
	subscriberJWTConfig jwtConfig
	publishOrigins      []string
	transport           Transport
	metrics             *Metrics
	topicSelectorStore  *TopicSelectorStore

	// Deprecated: use the Caddy server module or the standalone library instead
	config        *viper.Viper
	server        *http.Server
	metricsServer *http.Server
}

// WithTransport sets the transport to use.
func WithTransport(t Transport) Option {
	return func(h *Hub) {
		h.transport = t
	}
}

// WithTransportURL sets the transport to use by parsing the provided URL.
func WithTransportURL(tu string, l Logger) Option {
	u, err := url.Parse(tu)
	if err != nil {
		log.Panic(fmt.Errorf("transport_url: %w", err))
	}

	return func(h *Hub) {
		switch u.Scheme {
		case "null":
			h.transport = NewLocalTransport()

		case "bolt":
			transport, err := NewBoltTransport(u, l)
			if err != nil {
				log.Panic(err)
			}
			h.transport = transport

		default:
			log.Panic(&ErrInvalidTransportDSN{dsn: tu, msg: "no such transport available"})
		}
	}
}

func New(publisherJWTKey []byte, options ...Option) *Hub {
	if len(publisherJWTKey) == 0 {
		panic("providing a publisher JWT key is mandatory")
	}

	h := &Hub{
		publisherJWTConfig: jwtConfig{publisherJWTKey, jwt.SigningMethodHS256},
		topicSelectorStore: NewTopicSelectorStore(),
		writeTimeout:       600 * time.Second,
	}

	for _, o := range options {
		o(h)
	}

	if h.logger == nil {
		var (
			l   Logger
			err error
		)
		if h.debug {
			l, err = zap.NewDevelopment()
		} else {
			l, err = zap.NewProduction()
		}

		if err != nil {
			log.Panic(err)
		}

		h.logger = l
	}
	if h.transport == nil {
		h.transport = NewLocalTransport()
	}

	return h
}

// Stop stops disconnect all connected clients.
func (h *Hub) Stop() error {
	return h.transport.Close()
}
