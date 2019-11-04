package hub

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOptionsFromConfig(t *testing.T) {
	testEnv := map[string]string{
		"ACME_CERT_DIR":            "/tmp",
		"ACME_HOSTS":               "example.com example.org",
		"ACME_HTTP01_ADDR":         ":8080",
		"ADDR":                     "127.0.0.1:8080",
		"ALLOW_ANONYMOUS":          "1",
		"CERT_FILE":                "foo",
		"COMPRESS":                 "0",
		"CORS_ALLOWED_ORIGINS":     "*",
		"TRANSPORT_URL":            "bolt://test.db",
		"DEBUG":                    "1",
		"DEMO":                     "1",
		"KEY_FILE":                 "bar",
		"PUBLISHER_JWT_KEY":        "foo",
		"JWT_ALGORITHM":            "HS256",
		"PUBLISH_ALLOWED_ORIGINS":  "http://127.0.0.1:8080",
		"SUBSCRIBER_JWT_KEY":       "bar",
		"SUBSCRIBER_JWT_ALGORITHM": "HS256",
		"HEARTBEAT_INTERVAL":       "30s",
		"READ_TIMEOUT":             "1m",
		"WRITE_TIMEOUT":            "40s",
		"USE_FORWARDED_HEADERS":    "1",
	}
	for k, v := range testEnv {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	opts, err := NewOptionsFromConfig()
	require.Nil(t, err)
	assert.Equal(t, &Options{
		true,
		&url.URL{
			Scheme: "bolt",
			Host:   "test.db",
		},
		[]byte("foo"),
		[]byte("bar"),
		jwt.GetSigningMethod("HS256"),
		jwt.GetSigningMethod("HS256"),
		true,
		[]string{"*"},
		[]string{"http://127.0.0.1:8080"},
		"127.0.0.1:8080",
		[]string{"example.com", "example.org"},
		":8080",
		"/tmp",
		"foo",
		"bar",
		30 * time.Second,
		time.Minute,
		40 * time.Second,
		false,
		true,
		true,
	}, opts)
}

func TestMissingConfig(t *testing.T) {
	_, err := NewOptionsFromConfig()
	assert.EqualError(t, err, "The following configuration parameters must be defined: [publisher_jwt_key subscriber_jwt_key]")
}

func TestWrongPublisherAlgorithmEnv(t *testing.T) {
	testEnv := map[string]string{
		"PUBLISHER_JWT_KEY":        "foo",
		"PUBLISHER_JWT_ALGORITHM":  "FOO256",
		"SUBSCRIBER_JWT_KEY":       "bar",
		"SUBSCRIBER_JWT_ALGORITHM": "HS256",
	}
	for k, v := range testEnv {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	_, err := NewOptionsFromConfig()
	assert.EqualError(t, err, "publisher_jwt_algorithm: Invalid signing method <nil>")
}

func TestWrongSubscriberAlgorithmEnv(t *testing.T) {
	testEnv := map[string]string{
		"PUBLISHER_JWT_KEY":        "foo",
		"PUBLISHER_JWT_ALGORITHM":  "RS256",
		"SUBSCRIBER_JWT_KEY":       "bar",
		"SUBSCRIBER_JWT_ALGORITHM": "BAR256",
	}
	for k, v := range testEnv {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	_, err := NewOptionsFromConfig()
	assert.EqualError(t, err, "subscriber_jwt_algorithm: Invalid signing method <nil>")
}

func TestMissingKeyFile(t *testing.T) {
	os.Setenv("CERT_FILE", "foo")
	defer os.Unsetenv("CERT_FILE")

	_, err := NewOptionsFromConfig()
	assert.EqualError(t, err, "The following configuration parameters must be defined: [publisher_jwt_key subscriber_jwt_key key_file]")
}

func TestMissingCertFile(t *testing.T) {
	os.Setenv("KEY_FILE", "foo")
	defer os.Unsetenv("KEY_FILE")

	_, err := NewOptionsFromConfig()
	assert.EqualError(t, err, "The following configuration parameters must be defined: [publisher_jwt_key subscriber_jwt_key cert_file]")
}

func TestInvalidUrl(t *testing.T) {
	os.Setenv("TRANSPORT_URL", "http://[::1]%23")
	defer os.Unsetenv("TRANSPORT_URL")
	_, err := NewOptionsFromConfig()
	assert.EqualError(t, err, "transport_url: parse http://[::1]%23: invalid port \"%23\" after host")
}
