//go:build deprecated_transport

package caddy

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/mercure"
)

// Deprecated: use transports Caddy modules.
var transports = caddy.NewUsagePool() //nolint:gochecknoglobals
// Deprecated
//
//nolint:wrapcheck,ireturn
func (m *Mercure) createTransportDeprecated() (mercure.Transport, error) {
	if m.TransportURL == "" {
		return nil, nil
	}

	m.logger.Warn(`Setting the transport_url or the MERCURE_TRANSPORT_URL environment variable is deprecated, use the "transport" directive instead`)

	destructor, _, err := transports.LoadOrNew(m.TransportURL, func() (caddy.Destructor, error) {
		u, err := url.Parse(m.TransportURL)
		if err != nil {
			return nil, fmt.Errorf("invalid transport url: %w", err)
		}

		query := u.Query()
		if m.WriteTimeout != nil && !query.Has("write_timeout") {
			query.Set("write_timeout", time.Duration(*m.WriteTimeout).String())
		}

		if m.Subscriptions && !query.Has("subscriptions") {
			query.Set("subscriptions", "1")
		}

		u.RawQuery = query.Encode()

		transport, err := mercure.NewTransport(u, m.logger)
		if err != nil {
			return nil, err
		}

		return &TransportDestructor[mercure.Transport]{transport}, nil
	})
	if err != nil {
		return nil, err
	}

	return destructor.(*TransportDestructor[mercure.Transport]).Transport, nil
}

type deprecatedTransport struct {
	// Transport to use.
	//
	// Deprecated: use transports Caddy modules.
	TransportURL string `json:"transport_url,omitempty"`
}

func (m *Mercure) assignDeprecatedTransportURL(u string) {
	m.TransportURL = u
}

func (m *Mercure) assignDeprecatedTransportURLForEnv() {
	// BC layer with old versions of the built-in Caddyfile
	if m.TransportRaw != nil || m.TransportURL != "" {
		return
	}

	m.TransportURL = os.Getenv("MERCURE_TRANSPORT_URL")
}

//nolint:wrapcheck
func (m *Mercure) cleanupTransport() error {
	if m.TransportURL == "" {
		return nil
	}

	_, err := transports.Delete(m.TransportURL)

	return err
}
