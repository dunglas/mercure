//go:build !deprecated_transport

package caddy

import "github.com/dunglas/mercure"

func (_ *Mercure) createTransportDeprecated() (mercure.Transport, error) {
	return nil, nil
}

func (m *Mercure) assignDeprecatedTransportURL(_ string) {
}

func (m *Mercure) assignDeprecatedTransportURLForEnv() {
}

func (m *Mercure) cleanupTransportDeprecated() error {
	return nil
}

type deprecatedTransport struct{}
