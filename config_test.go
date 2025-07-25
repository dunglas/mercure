package mercure

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMissingConfig(t *testing.T) {
	t.Parallel()

	err := ValidateConfig(viper.New())
	require.EqualError(t, err, `invalid config: one of "jwt_key" or "publisher_jwt_key" configuration parameter must be defined`)
}

func TestMissingKeyFile(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("jwt_key", "abc")
	v.Set("cert_file", "foo")

	err := ValidateConfig(v)
	require.EqualError(t, err, `invalid config: if the "cert_file" configuration parameter is defined, "key_file" must be defined too`)
}

func TestMissingCertFile(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("jwt_key", "abc")
	v.Set("key_file", "foo")

	err := ValidateConfig(v)
	require.EqualError(t, err, `invalid config: if the "key_file" configuration parameter is defined, "cert_file" must be defined too`)
}

func TestSetFlags(t *testing.T) {
	t.Parallel()

	v := viper.New()
	fs := pflag.NewFlagSet("test", pflag.PanicOnError)
	SetFlags(fs, v)

	assert.Subset(t, v.AllKeys(), []string{"cert_file", "compress", "demo", "jwt_algorithm", "transport_url", "acme_hosts", "acme_cert_dir", "subscriber_jwt_key", "jwt_key", "allow_anonymous", "debug", "read_timeout", "publisher_jwt_algorithm", "write_timeout", "key_file", "use_forwarded_headers", "subscriber_jwt_algorithm", "addr", "publisher_jwt_key", "heartbeat_interval", "cors_allowed_origins", "publish_allowed_origins", "subscriptions", "dispatch_timeout"})
}

func TestInitConfig(t *testing.T) {
	t.Setenv("JWT_KEY", "foo")

	defer os.Unsetenv("JWT_KEY")

	v := viper.New()
	InitConfig(v)

	assert.Equal(t, "foo", v.GetString("jwt_key"))
}

func TestMetricsAreDisabledByDefault(t *testing.T) {
	t.Parallel()

	v := viper.New()
	SetConfigDefaults(v)

	assert.False(t, v.GetBool("metrics_enabled"))
}
