package hub

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigFromEnvVar(t *testing.T) {
	os.Setenv("JWT_KEY", "abc")
	defer os.Unsetenv("JWT_KEY")
	config, err := NewConfig()
	require.Nil(t, err)
	assert.Equal(t, "abc", config.GetString("jwt_key"))
}

func TestMissingConfig(t *testing.T) {
	_, err := NewConfig()
	assert.EqualError(t, err, `One of "jwt_key" or "publisher_jwt_key" configuration parameter must be defined`)
}

func TestMissingKeyFile(t *testing.T) {
	os.Setenv("JWT_KEY", "abc")
	os.Setenv("CERT_FILE", "foo")
	defer os.Unsetenv("CERT_FILE")

	_, err := NewConfig()
	assert.EqualError(t, err, `If the "cert_file" configuration parameter is defined, "key_file" must be defined too`)
}

func TestMissingCertFile(t *testing.T) {
	os.Setenv("JWT_KEY", "abc")
	os.Setenv("KEY_FILE", "foo")
	defer os.Unsetenv("KEY_FILE")

	_, err := NewConfig()
	assert.EqualError(t, err, `If the "key_file" configuration parameter is defined, "cert_file" must be defined too`)
}
