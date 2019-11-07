package hub

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestMissingConfig(t *testing.T) {
	err := ValidateConfig(viper.New())
	assert.EqualError(t, err, `one of "jwt_key" or "publisher_jwt_key" configuration parameter must be defined`)
}

func TestMissingKeyFile(t *testing.T) {
	v := viper.New()
	v.Set("jwt_key", "abc")
	v.Set("cert_file", "foo")

	err := ValidateConfig(v)
	assert.EqualError(t, err, `if the "cert_file" configuration parameter is defined, "key_file" must be defined too`)
}

func TestMissingCertFile(t *testing.T) {
	v := viper.New()
	v.Set("jwt_key", "abc")
	v.Set("key_file", "foo")

	err := ValidateConfig(v)
	assert.EqualError(t, err, `if the "key_file" configuration parameter is defined, "cert_file" must be defined too`)
}
