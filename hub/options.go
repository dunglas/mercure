package hub

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/spf13/viper"
)

// Options stores the hub's options
type Options struct {
	Debug                  bool
	TransportURL           *url.URL
	PublisherJWTKey        []byte
	SubscriberJWTKey       []byte
	PublisherJWTAlgorithm  jwt.SigningMethod
	SubscriberJWTAlgorithm jwt.SigningMethod
	AllowAnonymous         bool
	CorsAllowedOrigins     []string
	PublishAllowedOrigins  []string
	Addr                   string
	AcmeHosts              []string
	AcmeHTTP01Addr         string
	AcmeCertDir            string
	CertFile               string
	KeyFile                string
	HeartbeatInterval      time.Duration
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
	Compress               bool
	UseForwardedHeaders    bool
	Demo                   bool
}

// NewOptionsFromConfig creates a new option instance using Viper
// It returns an error if mandatory env env vars are missing
func NewOptionsFromConfig() (*Options, error) {
	var err error

	viper.SetConfigName("mercure")
	viper.AutomaticEnv()
	viper.AddConfigPath(".")
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = "$HOME/.config"
	}
	viper.AddConfigPath(configDir + "/mercure/")
	viper.AddConfigPath("/etc/mercure/")

	viper.SetDefault("debug", false)
	viper.SetDefault("transport_url", "bolt://updates.db")
	viper.SetDefault("jwt_algorithm", "HS512")
	viper.SetDefault("allow_anonymous", false)
	viper.SetDefault("acme_http01_addr", ":http")
	viper.SetDefault("heartbeat_interval", time.Duration(15*time.Second))
	viper.SetDefault("read_timeout", time.Duration(0))
	viper.SetDefault("write_timeout", time.Duration(0))
	viper.SetDefault("compress", false)
	viper.SetDefault("useForwarded_headers", false)
	viper.SetDefault("demo", false)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		// Fallback to environment variables
	}

	transportURL, err := parseTransportURLFromConfig()
	if err != nil {
		return nil, err
	}

	pubJwtAlgorithm := getJWTKeyAlgorithm("publisher")
	if _, ok := pubJwtAlgorithm.(jwt.SigningMethod); !ok {
		return nil, fmt.Errorf("publisher_jwt_algorithm: Invalid signing method %T", pubJwtAlgorithm)
	}

	subJwtAlgorithm := getJWTKeyAlgorithm("Subscriber")
	if _, ok := subJwtAlgorithm.(jwt.SigningMethod); !ok {
		return nil, fmt.Errorf("subscriber_jwt_algorithm: Invalid signing method %T", subJwtAlgorithm)
	}

	options := &Options{
		viper.GetBool("debug"),
		transportURL,
		[]byte(getJWTKey("publisher")),
		[]byte(getJWTKey("subscriber")),
		pubJwtAlgorithm,
		subJwtAlgorithm,
		viper.GetBool("allow_anonymous"),
		viper.GetStringSlice("cors_allowed_origins"),
		viper.GetStringSlice("publish_allowed_origins"),
		viper.GetString("addr"),
		viper.GetStringSlice("acme_hosts"),
		viper.GetString("acme_http01_addr"),
		viper.GetString("acme_cert_dir"),
		viper.GetString("cert_file"),
		viper.GetString("key_file"),
		viper.GetDuration("heartbeat_interval"),
		viper.GetDuration("read_timeout"),
		viper.GetDuration("write_timeout"),
		viper.GetBool("compress"),
		viper.GetBool("use_forwarded_headers"),
		viper.GetBool("demo") || viper.GetBool("debug"),
	}

	// TODO: Use Viper directly when https://github.com/spf13/viper/pull/573 will be merged
	missingEnv := make([]string, 0, 4)
	if len(options.PublisherJWTKey) == 0 {
		missingEnv = append(missingEnv, "publisher_jwt_key")
	}
	if len(options.SubscriberJWTKey) == 0 {
		missingEnv = append(missingEnv, "subscriber_jwt_key")
	}
	if len(options.CertFile) != 0 && len(options.KeyFile) == 0 {
		missingEnv = append(missingEnv, "key_file")
	}
	if len(options.KeyFile) != 0 && len(options.CertFile) == 0 {
		missingEnv = append(missingEnv, "cert_file")
	}

	if len(missingEnv) > 0 {
		return nil, fmt.Errorf("The following configuration parameters must be defined: %s", missingEnv)
	}

	return options, nil
}

func getJWTKey(role string) string {
	key := viper.GetString(fmt.Sprintf("%s_jwt_key", role))
	if key == "" {
		return viper.GetString("jwt_key")
	}

	return key
}

func getJWTKeyAlgorithm(role string) jwt.SigningMethod {
	keyType := viper.GetString(fmt.Sprintf("%s_jwt_algorithm", role))
	if keyType == "" {
		keyType = viper.GetString("jwt_algorithm")
	}

	return jwt.GetSigningMethod(keyType)
}

func parseTransportURLFromConfig() (*url.URL, error) {
	v := viper.GetString("transport_url")

	u, err := url.Parse(v)
	if err == nil {
		return u, nil
	}

	return nil, fmt.Errorf("transport_url: %w", err)
}
