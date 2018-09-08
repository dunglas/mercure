package hub

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOptionsFormNew(t *testing.T) {
	testEnv := map[string]string{
		"DEBUG":                "1",
		"PUBLISHER_JWT_KEY":    "foo",
		"SUBSCRIBER_JWT_KEY":   "bar",
		"ALLOW_ANONYMOUS":      "1",
		"CORS_ALLOWED_ORIGINS": "*",
		"ADDR":                 "127.0.0.1:8080",
		"ACME_HOSTS":           "example.com,example.org",
		"ACME_CERT_DIR":        "/tmp",
		"CERT_FILE":            "foo",
		"KEY_FILE":             "bar",
		"DEMO":                 "1",
	}
	for k, v := range testEnv {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	options, err := NewOptionsFromEnv()
	assert.Equal(t, &Options{
		true,
		[]byte("foo"),
		[]byte("bar"),
		true,
		[]string{"*"},
		"127.0.0.1:8080",
		[]string{"example.com", "example.org"},
		"/tmp",
		"foo",
		"bar",
		true,
	}, options)
	assert.Nil(t, err)
}

func TestMissingEnv(t *testing.T) {
	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "The following environment variable must be defined: [PUBLISHER_JWT_KEY SUBSCRIBER_JWT_KEY]")
}
