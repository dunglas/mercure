package hub

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// NewConfig create a Viper config
func NewConfig() (*viper.Viper, error) {
	v := viper.New()
	setConfigDefaults(v)

	v.SetConfigName("mercure")
	v.AutomaticEnv()
	v.AddConfigPath(".")
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = "$HOME/.config"
	}
	v.AddConfigPath(configDir + "/mercure/")
	v.AddConfigPath("/etc/mercure/")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		// Fallback to environment variables
	}

	if v.GetString("publisher_jwt_key") == "" && v.GetString("jwt_key") == "" {
		return nil, fmt.Errorf(`one of "jwt_key" or "publisher_jwt_key" configuration parameter must be defined`)
	}
	if v.GetString("cert_file") != "" && v.GetString("key_file") == "" {
		return nil, fmt.Errorf(`if the "cert_file" configuration parameter is defined, "key_file" must be defined too`)
	}
	if v.GetString("key_file") != "" && v.GetString("cert_file") == "" {
		return nil, fmt.Errorf(`if the "key_file" configuration parameter is defined, "cert_file" must be defined too`)
	}
	return v, nil
}

func setConfigDefaults(v *viper.Viper) {
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
