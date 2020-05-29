package hub

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ErrInvalidConfig is returned when the configuration is invalid.
var ErrInvalidConfig = errors.New("invalid config")

// SetConfigDefaults sets defaults on a Viper instance.
func SetConfigDefaults(v *viper.Viper) {
	v.SetDefault("debug", false)
	v.SetDefault("transport_url", "bolt://updates.db")
	v.SetDefault("jwt_algorithm", "HS256")
	v.SetDefault("allow_anonymous", false)
	v.SetDefault("acme_http01_addr", ":http")
	v.SetDefault("heartbeat_interval", 15*time.Second)
	v.SetDefault("read_timeout", 5*time.Second)
	v.SetDefault("write_timeout", 60*time.Second)
	v.SetDefault("dispatch_timeout", 5*time.Second)
	v.SetDefault("compress", false)
	v.SetDefault("use_forwarded_headers", false)
	v.SetDefault("demo", false)
	v.SetDefault("subscriptions", false)
	v.SetDefault("metrics", false)
	v.SetDefault("metrics_login", "mercure")
	v.SetDefault("metrics_password", "")
}

// ValidateConfig validates a Viper instance.
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
	if v.GetBool("metrics") {
		if v.GetString("metrics_login") != "" && v.GetString("metrics_password") == "" {
			return fmt.Errorf(`%w: if the "metrics_login" configuration parameter is defined, "metrics_password" must be defined too`, ErrInvalidConfig)
		}
		if v.GetString("metrics_password") != "" && v.GetString("metrics_login") == "" {
			return fmt.Errorf(`%w: if the "metrics_password" configuration parameter is defined, "metrics_login" must be defined too`, ErrInvalidConfig)
		}
	}
	return nil
}

// SetFlags creates flags and bind them to Viper.
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
	fs.StringP("log-format", "l", "", "the log format (JSON, FLUENTD or TEXT)")
	fs.BoolP("subscriptions", "s", false, "dispatch updates when subscriptions are created or terminated")
	fs.BoolP("metrics", "m", false, "enable metrics")
	fs.StringP("metrics_login", "", "mercure", "the user login allowed to access metrics")
	fs.StringP("metrics_password", "", "", "the user password allowed to access metrics")

	fs.VisitAll(func(f *pflag.Flag) {
		v.BindPFlag(strings.ReplaceAll(f.Name, "-", "_"), fs.Lookup(f.Name))
	})
}

// InitConfig reads in config file and ENV variables if set.
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
