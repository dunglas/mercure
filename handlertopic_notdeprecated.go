//go:build !deprecated_topic

package mercure

import "github.com/gorilla/mux"

// registerDeprecatedSubscriptionHandlers is the stub compiled without the
// deprecated_topic build tag: the v8 subscription routes are not in the
// binary.
func (h *Hub) registerDeprecatedSubscriptionHandlers(*mux.Router) bool {
	return false
}
