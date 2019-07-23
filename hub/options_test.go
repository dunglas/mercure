package hub

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewOptionsFormNew(t *testing.T) {
	testEnv := map[string]string{
		"ACME_CERT_DIR":             "/tmp",
		"ACME_HOSTS":                "example.com,example.org",
		"ADDR":                      "127.0.0.1:8080",
		"ALLOW_ANONYMOUS":           "1",
		"CERT_FILE":                 "foo",
		"COMPRESS":                  "0",
		"CORS_ALLOWED_ORIGINS":      "*",
		"DB_PATH":                   "test.db",
		"DEBUG":                     "1",
		"DEMO":                      "1",
		"HISTORY_SIZE":              "10",
		"HISTORY_CLEANUP_FREQUENCY": "0.3",
		"KEY_FILE":                  "bar",
		"PUBLISHER_JWT_KEY":         "foo",
		"PUBLISH_ALLOWED_ORIGINS":   "http://127.0.0.1:8080",
		"SUBSCRIBER_JWT_KEY":        "bar",
		"HEARTBEAT_INTERVAL":        "30s",
		"READ_TIMEOUT":              "1m",
		"WRITE_TIMEOUT":             "40s",
		"USE_FORWARDED_HEADERS":     "1",
	}
	for k, v := range testEnv {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	opts, err := NewOptionsFromEnv()
	assert.Equal(t, &Options{
		true,
		"test.db",
		10,
		0.3,
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
		30 * time.Second,
		time.Minute,
		40 * time.Second,
		false,
		true,
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

func TestInvalidDuration(t *testing.T) {
	vars := [3]string{"HEARTBEAT_INTERVAL", "READ_TIMEOUT", "WRITE_TIMEOUT"}
	for _, elem := range vars {
		os.Setenv(elem, "1 MN (invalid)")
		defer os.Unsetenv(elem)
		_, err := NewOptionsFromEnv()
		assert.EqualError(t, err, elem+": time: unknown unit  MN (invalid) in duration 1 MN (invalid)")

		os.Unsetenv(elem)
	}
}

func TestInvalidHistorySize(t *testing.T) {
	os.Setenv("HISTORY_SIZE", "invalid")
	defer os.Unsetenv("HISTORY_SIZE")

	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "HISTORY_SIZE: strconv.ParseUint: parsing \"invalid\": invalid syntax")
}

func TestInvalidHistoryCleanupFrequency(t *testing.T) {
	os.Setenv("HISTORY_CLEANUP_FREQUENCY", "invalid")
	defer os.Unsetenv("HISTORY_CLEANUP_FREQUENCY")

	_, err := NewOptionsFromEnv()
	assert.EqualError(t, err, "HISTORY_CLEANUP_FREQUENCY: strconv.ParseFloat: parsing \"invalid\": invalid syntax")
}
