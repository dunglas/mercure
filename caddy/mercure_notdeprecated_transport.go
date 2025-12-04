//go:build !deprecated_transport

package caddy

import (
	"os"

	"github.com/dunglas/mercure"
)

func (_ *Mercure) createTransportDeprecated() (mercure.Transport, error) {
	return nil, nil
}

func (m *Mercure) assignDeprecatedTransportURL(_ string) {
	m.logger.Error(`Setting the MERCURE_TRANSPORT_URL environment variable is not available anymore, use the "transport" directive instead`)
}

func (m *Mercure) assignDeprecatedTransportURLForEnv() {
	if "" != os.Getenv("MERCURE_TRANSPORT_URL") {
		m.logger.Error(`Setting the "transport_url" directive is not available anymore, use the "transport" directive instead`)
	}
}

func (m *Mercure) cleanupTransportDeprecated() error {
	return nil
}

type deprecatedTransport struct{}
