package hub

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// SetConfigDefaults sets defaults on a Viper instance
func SetConfigDefaults(v *viper.Viper) {
	v.SetDefault("debug", false)
	v.SetDefault("transport_url", "bolt://updates.db")
	v.SetDefault("jwt_algorithm", "HS256")
	v.SetDefault("allow_anonymous", false)
	v.SetDefault("acme_http01_addr", ":http")
	v.SetDefault("heartbeat_interval", 15*time.Second)
	v.SetDefault("read_timeout", time.Duration(0))
	v.SetDefault("write_timeout", time.Duration(0))
	v.SetDefault("compress", false)
	v.SetDefault("use_forwarded_headers", false)
	v.SetDefault("demo", false)
	v.SetDefault("dispatch_subscriptions", false)
	v.SetDefault("subscriptions_include_ip", false)
}

// ValidateConfig validates a Viper instance
func ValidateConfig(v *viper.Viper) error {
	if v.GetString("publisher_jwt_key") == "" && v.GetString("jwt_key") == "" {
		return fmt.Errorf(`one of "jwt_key" or "publisher_jwt_key" configuration parameter must be defined`)
	}
	if v.GetString("cert_file") != "" && v.GetString("key_file") == "" {
		return fmt.Errorf(`if the "cert_file" configuration parameter is defined, "key_file" must be defined too`)
	}
	if v.GetString("key_file") != "" && v.GetString("cert_file") == "" {
		return fmt.Errorf(`if the "key_file" configuration parameter is defined, "cert_file" must be defined too`)
	}
	return nil
}

// SetFlags creates flags and bind them to Viper
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
	fs.DurationP("read-timeout", "R", time.Duration(0), "maximum duration for reading the entire request, including the body")
	fs.DurationP("write-timeout", "W", time.Duration(0), "maximum duration before timing out writes of the response")
	fs.BoolP("compress", "Z", false, "enable or disable HTTP compression support")
	fs.BoolP("use-forwarded-headers", "f", false, "enable headers forwarding")
	fs.BoolP("demo", "D", false, "enable the demo mode")
	fs.StringP("log-format", "l", "", "the log format (JSON, FLUENTD or TEXT)")
	fs.BoolP("dispatch-subscriptions", "s", false, "dispatch updates when subscriptions are created or terminated")
	fs.BoolP("subscriptions-include-ip", "I", false, "include the IP address of the subscriber in the subscription update")

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
