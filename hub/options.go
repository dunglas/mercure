package hub

import (
	"fmt"
	"os"
	"strings"
)

// Options stores the hub's options
type Options struct {
	Debug              bool
	PublisherJWTKey    []byte
	SubscriberJWTKey   []byte
	AllowAnonymous     bool
	Addr               string
	CorsAllowedOrigins []string
	Demo               bool
}

// NewOptionsFromEnv creates a new option instance from environment
// It return an error if mandatory env env vars are missing
func NewOptionsFromEnv() (*Options, error) {
	listen := os.Getenv("ADDR")
	if listen == "" {
		listen = ":80"
	}

	options := &Options{
		os.Getenv("DEBUG") == "1",
		[]byte(os.Getenv("PUBLISHER_JWT_KEY")),
		[]byte(os.Getenv("SUBSCRIBER_JWT_KEY")),
		os.Getenv("ALLOW_ANONYMOUS") == "1",
		listen,
		strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ","),
		os.Getenv("DEMO") == "1" || os.Getenv("DEBUG") == "1",
	}

	missingEnv := make([]string, 0, 2)
	if len(options.PublisherJWTKey) == 0 {
		missingEnv = append(missingEnv, "PUBLISHER_JWT_KEY")
	}
	if len(options.SubscriberJWTKey) == 0 {
		missingEnv = append(missingEnv, "SUBSCRIBER_JWT_KEY")
	}

	if len(missingEnv) > 0 {
		return nil, fmt.Errorf("The following environment variable must be defined: %s", missingEnv)
	}

	return options, nil
}
