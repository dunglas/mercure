//go:build deprecated_server

package mercure

import (
	"net/http"

	"github.com/spf13/viper"
)

// Deprecated: use the Caddy server module or the standalone library instead.
type deprecatedHub struct {
	config        *viper.Viper
	server        *http.Server
	metricsServer *http.Server
}
