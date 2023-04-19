package mercure

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// ErrInvalidConfig is returned when the configuration is invalid.
//
// Deprecated: use the Caddy server module or the standalone library instead.
var ErrInvalidConfig = errors.New("invalid config")

// SetConfigDefaults sets defaults on a Viper instance.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func SetConfigDefaults(v *viper.Viper) {
	v.SetDefault("debug", false)
	v.SetDefault("transport_url", "bolt://updates.db")
	v.SetDefault("jwt_algorithm", "HS256")
	v.SetDefault("allow_anonymous", false)
	v.SetDefault("acme_http01_addr", ":http")
	v.SetDefault("heartbeat_interval", 40*time.Second) // Must be < 45s for compatibility with Yaffle/EventSource
	v.SetDefault("read_timeout", 5*time.Second)
	v.SetDefault("read_header_timeout", 3*time.Second)
	v.SetDefault("write_timeout", 600*time.Second)
	v.SetDefault("dispatch_timeout", 5*time.Second)
	v.SetDefault("compress", false)
	v.SetDefault("use_forwarded_headers", false)
	v.SetDefault("demo", false)
	v.SetDefault("subscriptions", false)

	v.SetDefault("metrics_enabled", false)
	v.SetDefault("metrics_addr", "127.0.0.1:9764")
}

// ValidateConfig validates a Viper instance.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func ValidateConfig(v *viper.Viper) error {
	if v.GetString("publisher_jwt_key") == "" && v.GetString("jwt_key") == "" {
		return fmt.Errorf(`%w: one of "jwt_key" or "publisher_jwt_key" configuration parameter must be defined`, ErrInvalidConfig)
	}
	if v.GetString("cert_file") != "" && v.GetString("key_file") == "" {
		return fmt.Errorf(`%w: if the "cert_file" configuration parameter is defined, "key_file" must be defined too`, ErrInvalidConfig)
	}
	if v.GetString("key_file") != "" && v.GetString("cert_file") == "" {
		return fmt.Errorf(`%w: if the "key_file" configuration parameter is defined, "cert_file" must be defined too`, ErrInvalidConfig)
	}
	if !v.GetBool("metrics_enabled") {
		return nil
	}

	if v.GetString("metrics_addr") == "" {
		return fmt.Errorf(`%w: "metrics_addr" must be defined when metrics is enabled`, ErrInvalidConfig)
	}
	if v.GetString("metrics_addr") == v.GetString("addr") {
		return fmt.Errorf(`%w: "metrics_addr" must not be the same as "addr"`, ErrInvalidConfig)
	}

	return nil
}

// SetFlags creates flags and bind them to Viper.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func SetFlags(fs *pflag.FlagSet, v *viper.Viper) {
	fs.BoolP("debug", "d", false, "enable the debug mode")
	fs.StringP("transport-url", "t", "", "transport and history system to use")
	fs.StringP("jwt-key", "k", "", "JWT key")
	fs.StringP("jwt-algorithm", "O", "", "JWT algorithm")
	fs.StringP("publisher-jwt-key", "K", "", "publisher JWT key")
	fs.StringP("publisher-jwt-algorithm", "A", "", "publisher JWT algorithm")
	fs.StringP("subscriber-jwt-key", "L", "", "subscriber JWT key")
	fs.StringP("subscriber-jwt-algorithm", "B", "", "subscriber JWT algorithm")
	fs.BoolP("allow-anonymous", "X", false, "allow subscribers with no valid JWT to connect")
	fs.StringSliceP("cors-allowed-origins", "c", []string{}, "list of allowed CORS origins")
	fs.StringSliceP("publish-allowed-origins", "p", []string{}, "list of origins allowed to publish")
	fs.StringP("addr", "a", "", "the address to listen on")
	fs.StringSliceP("acme-hosts", "o", []string{}, "list of hosts for which Let's Encrypt certificates must be issued")
	fs.StringP("acme-cert-dir", "E", "", "the directory where to store Let's Encrypt certificates")
	fs.StringP("cert-file", "C", "", "a cert file (to use a custom certificate)")
	fs.StringP("key-file", "J", "", "a key file (to use a custom certificate)")
	fs.DurationP("heartbeat-interval", "i", 15*time.Second, "interval between heartbeats (0s to disable)")
	fs.DurationP("read-timeout", "R", 5*time.Second, "maximum duration for reading the entire request, including the body, 5s by default, 0s to disable")
	fs.DurationP("write-timeout", "W", 60*time.Second, "maximum duration of a connection, 60s by default, 0s to disable")
	fs.DurationP("dispatch-timeout", "T", 5*time.Second, "maximum duration of the dispatch of a single update, 5s by default, 0s to disable")
	fs.BoolP("compress", "Z", false, "enable or disable HTTP compression support")
	fs.BoolP("use-forwarded-headers", "f", false, "enable headers forwarding")
	fs.BoolP("demo", "D", false, "enable the demo mode")
	fs.BoolP("subscriptions", "s", false, "dispatch updates when subscriptions are created or terminated")
	fs.Int64("tcsz", DefaultTopicSelectorStoreLRUMaxEntriesPerShard, "size of each shard in topic selector store cache")

	fs.Bool("metrics-enabled", false, "enable metrics")
	fs.String("metrics-addr", "127.0.0.1:9764", "metrics HTTP server address")

	fs.VisitAll(func(f *pflag.Flag) {
		v.BindPFlag(strings.ReplaceAll(f.Name, "-", "_"), fs.Lookup(f.Name))
	})
}

// InitConfig reads in config file and ENV variables if set.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func InitConfig(v *viper.Viper) {
	SetConfigDefaults(v)

	v.SetConfigName("mercure")
	v.AutomaticEnv()

	v.AddConfigPath(".")
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = "$HOME/.config"
	}
	v.AddConfigPath(configDir + "/mercure/")
	v.AddConfigPath("/etc/mercure/")

	v.ReadInConfig()
}

// NewHubFromViper creates a new Hub from the Viper config.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func NewHubFromViper(v *viper.Viper) (*Hub, error) { //nolint:funlen,gocognit
	if err := ValidateConfig(v); err != nil {
		log.Panic(err)
	}

	options := []Option{}
	var (
		logger Logger
		err    error
		k      string
	)
	if v.GetBool("debug") {
		options = append(options, WithDebug())
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}

	if err != nil {
		return nil, fmt.Errorf("unable to create logger: %w", err)
	}

	var tss *TopicSelectorStore
	tcsz := v.GetInt64("tcsz")
	if tcsz == 0 {
		tcsz = DefaultTopicSelectorStoreLRUMaxEntriesPerShard
	}
	tss, err = NewTopicSelectorStoreLRU(tcsz, DefaultTopicSelectorStoreLRUShardCount)
	if err != nil {
		return nil, err
	}

	if t := v.GetString("transport_url"); t != "" {
		u, err := url.Parse(t)
		if err != nil {
			return nil, fmt.Errorf("invalid transport url: %w", err)
		}

		t, err := NewTransport(u, logger)
		if err != nil {
			return nil, err
		}

		options = append(options, WithTransport(t))
	}

	if v.GetBool("metrics_enabled") {
		options = append(options, WithMetrics(NewPrometheusMetrics(nil)))
	}

	options = append(options, WithLogger(logger), WithTopicSelectorStore(tss))
	if v.GetBool("allow_anonymous") {
		options = append(options, WithAnonymous())
	}
	if v.GetBool("demo") {
		options = append(options, WithDemo())
	}
	if d := v.GetDuration("write_timeout"); d != 600*time.Second {
		options = append(options, WithWriteTimeout(d))
	}
	if d := v.GetDuration("dispatch_timeout"); d != 0 {
		options = append(options, WithDispatchTimeout(d))
	}
	if v.GetBool("subscriptions") {
		options = append(options, WithSubscriptions())
	}
	if d := v.GetDuration("heartbeat_interval"); d != 0 {
		options = append(options, WithHeartbeat(d))
	}
	if k = v.GetString("publisher_jwt_key"); k == "" {
		k = v.GetString("jwt_key")
	}
	if k != "" {
		alg := v.GetString("publisher_jwt_algorithm")
		if alg == "" {
			if alg = v.GetString("jwt_algorithm"); alg == "" {
				alg = "HS256"
			}
		}

		options = append(options, WithPublisherJWT([]byte(k), alg))
	}
	if k = v.GetString("subscriber_jwt_key"); k == "" {
		k = v.GetString("jwt_key")
	}
	if k != "" {
		alg := v.GetString("subscriber_jwt_algorithm")
		if alg == "" {
			if alg = v.GetString("jwt_algorithm"); alg == "" {
				alg = "HS256"
			}
		}

		options = append(options, WithSubscriberJWT([]byte(k), alg))
	}
	if h := v.GetStringSlice("acme_hosts"); len(h) > 0 {
		options = append(options, WithAllowedHosts(h))
	}
	if o := v.GetStringSlice("publish_allowed_origins"); len(o) > 0 {
		options = append(options, WithPublishOrigins(o))
	}
	if o := v.GetStringSlice("cors_allowed_origins"); len(o) > 0 {
		options = append(options, WithCORSOrigins(o))
	}

	h, err := NewHub(options...)
	if err != nil {
		return nil, err
	}
	h.config = v

	return h, err
}

// Start is an helper method to start the Mercure Hub.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func Start() {
	h, err := NewHubFromViper(viper.GetViper())
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if err := h.transport.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	h.Serve()
}
