package hub

import (
	"fmt"
	"time"

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
