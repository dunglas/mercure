//go:build deprecated_transport

package mercure

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"
)

var (
	// Deprecated: directly instantiate the transport or use transports Caddy modules.
	transportFactories   = make(map[string]TransportFactory) //nolint:gochecknoglobals
	transportFactoriesMu sync.RWMutex                        //nolint:gochecknoglobals
)

func init() { //nolint:gochecknoinits
	//mercure:deadlock
	RegisterTransportFactory("bolt", DeprecatedNewBoltTransport)
	RegisterTransportFactory("local", DeprecatedNewLocalTransport)
}

// Deprecated: TransportFactory is the factory to initialize a new transport.
type TransportFactory = func(u *url.URL, l Logger) (Transport, error)

// Deprecated: directly instantiate the transport or use transports Caddy modules.
func NewTransport(u *url.URL, l Logger) (Transport, error) { //nolint:ireturn
	transportFactoriesMu.RLock()

	f, ok := transportFactories[u.Scheme]

	transportFactoriesMu.RUnlock()

	if !ok {
		return nil, &TransportError{dsn: u.Redacted(), msg: "no such transport available"}
	}

	return f(u, l)
}

// Deprecated: directly instantiate the transport or use transports Caddy modules.
func RegisterTransportFactory(scheme string, factory TransportFactory) {
	transportFactoriesMu.Lock()

	transportFactories[scheme] = factory

	transportFactoriesMu.Unlock()
}

// DeprecatedNewBoltTransport creates a new BoltTransport.
//
// Deprecated: use NewBoltTransport() instead.
func DeprecatedNewBoltTransport(u *url.URL, l Logger) (Transport, error) { //nolint:ireturn
	var err error

	q := u.Query()
	bucketName := defaultBoltBucketName

	if q.Get("bucket_name") != "" {
		bucketName = q.Get("bucket_name")
	}

	size := uint64(0)
	if sizeParameter := q.Get("size"); sizeParameter != "" {
		size, err = strconv.ParseUint(sizeParameter, 10, 64)
		if err != nil {
			return nil, &TransportError{u.Redacted(), fmt.Sprintf(`invalid "size" parameter %q`, sizeParameter), err}
		}
	}

	cleanupFrequency := BoltDefaultCleanupFrequency
	cleanupFrequencyParameter := q.Get("cleanup_frequency")

	if cleanupFrequencyParameter != "" {
		cleanupFrequency, err = strconv.ParseFloat(cleanupFrequencyParameter, 64)
		if err != nil {
			return nil, &TransportError{u.Redacted(), fmt.Sprintf(`invalid "cleanup_frequency" parameter %q`, cleanupFrequencyParameter), err}
		}
	}

	path := u.Path // absolute path (bolt:///path.db)

	if path == "" {
		path = u.Host // relative path (bolt://path.db)
	}

	if path == "" {
		return nil, &TransportError{u.Redacted(), "missing path", err}
	}

	return NewBoltTransport(NewSubscriberList(DefaultSubscriberListCacheSize), l, path, bucketName, size, cleanupFrequency)
}

// DeprecatedNewLocalTransport creates a new LocalTransport.
//
// Deprecated: use NewLocalTransport() instead.
func DeprecatedNewLocalTransport(_ *url.URL, _ Logger) (Transport, error) { //nolint:ireturn
	return NewLocalTransport(NewSubscriberList(DefaultSubscriberListCacheSize)), nil
}
