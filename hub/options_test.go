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

func TestNewOptionsFormNew(t *testing.T) {
	testEnv := map[string]string{
		"ACME_CERT_DIR":            "/tmp",
		"ACME_HOSTS":               "example.com,example.org",
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
		"PUBLISHER_JWT_ALGORITHM":  "HS256",
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

	opts, err := NewOptionsFromEnv()
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

func TestMissingEnv(t *testing.T) {
	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "The following environment variable must be defined: [PUBLISHER_JWT_KEY SUBSCRIBER_JWT_KEY]")
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

	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "Expected valid signing method for 'PUBLISHER_JWT_ALGORITHM', got <nil>")
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

	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "Expected valid signing method for 'SUBSCRIBER_JWT_ALGORITHM', got <nil>")
}

func TestMissingKeyFile(t *testing.T) {
	os.Setenv("CERT_FILE", "foo")
	defer os.Unsetenv("CERT_FILE")

	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "The following environment variable must be defined: [PUBLISHER_JWT_KEY SUBSCRIBER_JWT_KEY KEY_FILE]")
}

func TestMissingCertFile(t *testing.T) {
	os.Setenv("KEY_FILE", "foo")
	defer os.Unsetenv("KEY_FILE")

	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "The following environment variable must be defined: [PUBLISHER_JWT_KEY SUBSCRIBER_JWT_KEY CERT_FILE]")
}

func TestInvalidDuration(t *testing.T) {
	vars := [3]string{"HEARTBEAT_INTERVAL", "READ_TIMEOUT", "WRITE_TIMEOUT"}
	for _, elem := range vars {
		os.Setenv(elem, "1 MN (invalid)")
		_, err := NewOptionsFromEnv()
		assert.EqualError(t, err, elem+": time: unknown unit  MN (invalid) in duration 1 MN (invalid)")

		os.Unsetenv(elem)
	}
}

func TestInvalidUrl(t *testing.T) {
	vars := []string{"TRANSPORT_URL"}
	for _, elem := range vars {
		os.Setenv(elem, "http://[::1]%23")
		defer os.Unsetenv(elem)
		_, err := NewOptionsFromEnv()
		assert.EqualError(t, err, elem+": parse http://[::1]%23: invalid port \"%23\" after host")
	}
}
