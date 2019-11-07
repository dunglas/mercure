package hub

import (
	"fmt"
	"os"
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
}

// ValidateConfig validates a Viper instance
func ValidateConfig(v *viper.Viper) error {
	if v.GetString("publisher_jwt_key") == "" && v.GetString("jwt_key") == "" {
		return fmt.Errorf(`one of "jwt_key" or "publisher_jwt_key" configuration parameter must be defined`)
	}
	if v.GetString("cert_file") != "" && v.GetString("key_file") == "" {
		return fmt.Errorf(`ff the "cert_file" configuration parameter is defined, "key_file" must be defined too`)
	}
	if v.GetString("key_file") != "" && v.GetString("cert_file") == "" {
		return fmt.Errorf(`ff the "key_file" configuration parameter is defined, "cert_file" must be defined too`)
	}
	return nil
}

// SetFlags creates flags and bind them to Viper
func SetFlags(fs *pflag.FlagSet, v *viper.Viper) {
	fs.BoolP("debug", "d", false, "enable the debug mode")
	v.BindPFlag("debug", fs.Lookup("debug"))

	fs.StringP("transport-url", "t", "", "transport and history system to use")
	v.BindPFlag("transport_url", fs.Lookup("transport-url"))

	fs.StringP("jwt-key", "k", "", "JWT key")
	v.BindPFlag("jwt_key", fs.Lookup("jwt-key"))

	fs.StringP("jwt-algorithm", "O", "", "JWT algorithm")
	v.BindPFlag("jwt_algorithm", fs.Lookup("jwt-algorithm"))

	fs.StringP("publisher-jwt-key", "K", "", "publisher JWT key")
	v.BindPFlag("publisher_jwt_key", fs.Lookup("publisher-jwt-key"))

	fs.StringP("publisher-jwt-algorithm", "A", "", "publisher JWT algorithm")
	v.BindPFlag("publisher_jwt_algorithm", fs.Lookup("publisher-jwt-algorithm"))

	fs.StringP("subscriber-jwt-key", "L", "", "subscriber JWT key")
	v.BindPFlag("subscriber_jwt_key", fs.Lookup("subscriber-jwt-key"))

	fs.StringP("subscriber-jwt-algorithm", "B", "", "subscriber JWT algorithm")
	v.BindPFlag("subscriber_jwt_algorithm", fs.Lookup("subscriber-jwt-algorithm"))

	fs.BoolP("allow-anonymous", "X", false, "allow subscribers with no valid JWT to connect")
	v.BindPFlag("allow_anonymous", fs.Lookup("allow-anonymous"))

	fs.StringSliceP("cors-allowed-origins", "c", []string{}, "list of allowed CORS origins")
	v.BindPFlag("cors_allowed_origins", fs.Lookup("cors-allowed-origins"))

	fs.StringSliceP("publish-allowed-origins", "p", []string{}, "list of origins allowed to publish")
	v.BindPFlag("publish_allowed_origins", fs.Lookup("publish-allowed-origins"))

	fs.StringP("addr", "a", "", "the address to listen on")
	v.BindPFlag("addr", fs.Lookup("addr"))

	fs.StringSliceP("acme-hosts", "o", []string{}, "list of hosts for which Let's Encrypt certificates must be issued")
	v.BindPFlag("acme_hosts", fs.Lookup("acme-hosts"))

	fs.StringP("acme-cert-dir", "E", "", "the directory where to store Let's Encrypt certificates")
	v.BindPFlag("acme_cert_dir", fs.Lookup("acme-cert-dir"))

	fs.StringP("cert-file", "C", "", "a cert file (to use a custom certificate)")
	v.BindPFlag("cert_file", fs.Lookup("cert-file"))

	fs.StringP("key-file", "J", "", "a key file (to use a custom certificate)")
	v.BindPFlag("key_file", fs.Lookup("key-file"))

	fs.StringP("heartbeat-interval", "i", "", "interval between heartbeats (0s to disable)")
	v.BindPFlag("heartbeat_interval", fs.Lookup("heartbeat-interval"))

	fs.StringP("read-timeout", "R", "", "maximum duration for reading the entire request, including the body")
	v.BindPFlag("read_timeout", fs.Lookup("read-timeout"))

	fs.StringP("write-timeout", "W", "", "maximum duration before timing out writes of the response")
	v.BindPFlag("write_timeout", fs.Lookup("write-timeout"))

	fs.BoolP("compress", "Z", false, "enable or disable HTTP compression support")
	v.BindPFlag("compress", fs.Lookup("compress"))

	fs.BoolP("use-forwarded-headers", "f", false, "enable headers forwarding")
	v.BindPFlag("use_forwarded_headers", fs.Lookup("use-forwarded-headers"))

	fs.BoolP("demo", "D", false, "enable the demo mode")
	v.BindPFlag("demo", fs.Lookup("demo"))

	fs.StringP("log-format", "l", "", "the log format (JSON, FLUENTD or TEXT)")
	v.BindPFlag("log_format", fs.Lookup("log-format"))
}

// InitConfig reads in config file and ENV variables if set.
func InitConfig() {
	v := viper.GetViper()
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
