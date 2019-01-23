package hub

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOptionsFormNew(t *testing.T) {
	testEnv := map[string]string{
		"ACME_CERT_DIR":           "/tmp",
		"ACME_HOSTS":              "example.com,example.org",
		"ADDR":                    "127.0.0.1:8080",
		"ALLOW_ANONYMOUS":         "1",
		"CERT_FILE":               "foo",
		"CORS_ALLOWED_ORIGINS":    "*",
		"DB_PATH":                 "test.db",
		"DEBUG":                   "1",
		"DEMO":                    "1",
		"KEY_FILE":                "bar",
		"PUBLISHER_JWT_KEY":       "foo",
		"PUBLISH_ALLOWED_ORIGINS": "http://127.0.0.1:8080",
		"SUBSCRIBER_JWT_KEY":      "bar",
	}
	for k, v := range testEnv {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	opts, err := NewOptionsFromEnv()
	assert.Equal(t, &Options{
		true,
		"test.db",
		[]byte("foo"),
		[]byte("bar"),
		true,
		[]string{"*"},
		[]string{"http://127.0.0.1:8080"},
		"127.0.0.1:8080",
		[]string{"example.com", "example.org"},
		"/tmp",
		"foo",
		"bar",
		true,
	}, opts)
	assert.Nil(t, err)
}

func TestMissingEnv(t *testing.T) {
	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "The following environment variable must be defined: [PUBLISHER_JWT_KEY SUBSCRIBER_JWT_KEY]")
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
