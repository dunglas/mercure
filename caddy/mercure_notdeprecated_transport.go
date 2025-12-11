//go:build !deprecated_transport

package caddy

import (
	"os"

	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/mercure"
)

func (_ *Mercure) createTransportDeprecated() (mercure.Transport, error) {
	return nil, nil
}

func (m *Mercure) assignDeprecatedTransportURL(_ string) {
	caddy.Log().Error(`Setting the "transport_url" directive is not available anymore, use the "transport" directive instead`)
}

func (m *Mercure) assignDeprecatedTransportURLForEnv() {
	if "" != os.Getenv("MERCURE_TRANSPORT_URL") {
		caddy.Log().Error(`The "MERCURE_TRANSPORT_URL"" environment variable is not supported anymore, set the "transport" directive in the "MERCURE_EXTRA_DIRECTIVES" environment variable instead`)
	}
}

func (m *Mercure) cleanupTransportDeprecated() error {
	return nil
}

type deprecatedTransport struct{}
