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
	CorsAllowedOrigins []string
	Addr               string
	AcmeHosts          []string
	AcmeCertDir        string
	CertFile           string
	KeyFile            string
	Demo               bool
}

// NewOptionsFromEnv creates a new option instance from environment
// It return an error if mandatory env env vars are missing
func NewOptionsFromEnv() (*Options, error) {
	options := &Options{
		os.Getenv("DEBUG") == "1",
		[]byte(os.Getenv("PUBLISHER_JWT_KEY")),
		[]byte(os.Getenv("SUBSCRIBER_JWT_KEY")),
		os.Getenv("ALLOW_ANONYMOUS") == "1",
		splitVar(os.Getenv("CORS_ALLOWED_ORIGINS")),
		os.Getenv("ADDR"),
		splitVar(os.Getenv("ACME_HOSTS")),
		os.Getenv("ACME_CERT_DIR"),
		os.Getenv("CERT_FILE"),
		os.Getenv("KEY_FILE"),
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

func splitVar(v string) []string {
	if v == "" {
		return []string{}
	}

	return strings.Split(v, ",")
}
