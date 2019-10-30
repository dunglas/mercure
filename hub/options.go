package hub

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Options stores the hub's options
type Options struct {
	Debug                   bool
	DBPath                  string
	HistorySize             uint64
	HistoryCleanupFrequency float64
	PublisherJWTKey         []byte
	SubscriberJWTKey        []byte
	PublisherJWTAlgorithm   jwt.SigningMethod
	SubscriberJWTAlgorithm  jwt.SigningMethod
	AllowAnonymous          bool
	CorsAllowedOrigins      []string
	PublishAllowedOrigins   []string
	Addr                    string
	AcmeHosts               []string
	AcmeHTTP01Addr          string
	AcmeCertDir             string
	CertFile                string
	KeyFile                 string
	HeartbeatInterval       time.Duration
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	Compress                bool
	UseForwardedHeaders     bool
	Demo                    bool
}

func getJWTKey(role string) string {
	key := os.Getenv(fmt.Sprintf("%s_JWT_KEY", role))
	if key == "" {
		return os.Getenv("JWT_KEY")
	}

	return key
}

func getJWTKeyAlgorithm(role string) jwt.SigningMethod {
	keyType := os.Getenv(fmt.Sprintf("%s_JWT_ALGORITHM", role))

	if keyType == "" {
		keyType = os.Getenv("JWT_ALGORITHM")
	}

	if keyType == "" {
		keyType = "HS512"
	}

	signingMethod := jwt.GetSigningMethod(keyType)

	return signingMethod
}

// NewOptionsFromEnv creates a new option instance from environment
// It returns an error if mandatory env env vars are missing
func NewOptionsFromEnv() (*Options, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "updates.db"
	}

	var err error

	historySize := uint64(0)
	historySizeFromEnv := os.Getenv("HISTORY_SIZE")
	if historySizeFromEnv != "" {
		historySize, err = strconv.ParseUint(historySizeFromEnv, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("HISTORY_SIZE: %s", err)
		}
	}

	historyCleanupFrequency := 0.3
	historyCleanupFrequencyFromEnv := os.Getenv("HISTORY_CLEANUP_FREQUENCY")
	if historyCleanupFrequencyFromEnv != "" {
		historyCleanupFrequency, err = strconv.ParseFloat(historyCleanupFrequencyFromEnv, 64)
		if err != nil {
			return nil, fmt.Errorf("HISTORY_CLEANUP_FREQUENCY: %s", err)
		}
	}

	heartbeatInterval, err := parseDurationFromEnvVar("HEARTBEAT_INTERVAL", time.Duration(15*time.Second))
	if err != nil {
		return nil, err
	}

	readTimeout, err := parseDurationFromEnvVar("READ_TIMEOUT", time.Duration(0))
	if err != nil {
		return nil, err
	}

	writeTimeout, err := parseDurationFromEnvVar("WRITE_TIMEOUT", time.Duration(0))
	if err != nil {
		return nil, err
	}

	pubJwtAlgorithm := getJWTKeyAlgorithm("PUBLISHER")
	subJwtAlgorithm := getJWTKeyAlgorithm("SUBSCRIBER")

	if _, ok := pubJwtAlgorithm.(jwt.SigningMethod); !ok {
		return nil, fmt.Errorf("Expected valid signing method for 'PUBLISHER_JWT_ALGORITHM', got %T", pubJwtAlgorithm)
	}

	if _, ok := subJwtAlgorithm.(jwt.SigningMethod); !ok {
		return nil, fmt.Errorf("Expected valid signing method for 'SUBSCRIBER_JWT_ALGORITHM', got %T", subJwtAlgorithm)
	}

	acmeHTTP01Addr := os.Getenv("ACME_HTTP01_ADDR")
	if acmeHTTP01Addr == "" {
		acmeHTTP01Addr = ":http"
	}

	options := &Options{
		os.Getenv("DEBUG") == "1",
		dbPath,
		historySize,
		historyCleanupFrequency,
		[]byte(getJWTKey("PUBLISHER")),
		[]byte(getJWTKey("SUBSCRIBER")),
		pubJwtAlgorithm,
		subJwtAlgorithm,
		os.Getenv("ALLOW_ANONYMOUS") == "1",
		splitVar(os.Getenv("CORS_ALLOWED_ORIGINS")),
		splitVar(os.Getenv("PUBLISH_ALLOWED_ORIGINS")),
		os.Getenv("ADDR"),
		splitVar(os.Getenv("ACME_HOSTS")),
		acmeHTTP01Addr,
		os.Getenv("ACME_CERT_DIR"),
		os.Getenv("CERT_FILE"),
		os.Getenv("KEY_FILE"),
		heartbeatInterval,
		readTimeout,
		writeTimeout,
		os.Getenv("COMPRESS") != "0",
		os.Getenv("USE_FORWARDED_HEADERS") == "1",
		os.Getenv("DEMO") == "1" || os.Getenv("DEBUG") == "1",
	}

	missingEnv := make([]string, 0, 4)
	if len(options.PublisherJWTKey) == 0 {
		missingEnv = append(missingEnv, "PUBLISHER_JWT_KEY")
	}
	if len(options.SubscriberJWTKey) == 0 {
		missingEnv = append(missingEnv, "SUBSCRIBER_JWT_KEY")
	}
	if len(options.CertFile) != 0 && len(options.KeyFile) == 0 {
		missingEnv = append(missingEnv, "KEY_FILE")
	}
	if len(options.KeyFile) != 0 && len(options.CertFile) == 0 {
		missingEnv = append(missingEnv, "CERT_FILE")
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

func parseDurationFromEnvVar(k string, d time.Duration) (time.Duration, error) {
	v := os.Getenv(k)
	if v == "" {
		return d, nil
	}

	dur, err := time.ParseDuration(v)
	if err == nil {
		return dur, nil
	}

	return time.Duration(0), fmt.Errorf("%s: %s", k, err)
}
