package mercure

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// protectedResourceMetadataPath is the RFC 9728 well-known location for the
// hub's OAuth 2.0 protected resource metadata, derived from the hub URL.
const protectedResourceMetadataPath = "/.well-known/oauth-protected-resource" + defaultHubURL

// protectedResourceMetadata is the subset of OAuth 2.0 Protected Resource
// Metadata (RFC 9728) the hub advertises. jwks_uri is intentionally omitted:
// the hub hosts no JWKS endpoint, and a single jwks_uri cannot represent the
// separate publisher and subscriber key sets.
type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
}

// bearerMethodsSupported lists the token presentation methods the hub accepts.
// "mercureCookie" is a Mercure extension to RFC 6750; the namespaced name
// avoids colliding with any future IANA-registered "cookie" method.
//
//nolint:gochecknoglobals
var bearerMethodsSupported = []string{"header", "query", "mercureCookie"}

// ProtectedResourceMetadataHandler serves the hub's RFC 9728 protected
// resource metadata document.
func (h *Hub) ProtectedResourceMetadataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	metadata := protectedResourceMetadata{
		Resource:               h.resourceIdentifier,
		BearerMethodsSupported: bearerMethodsSupported,
		AuthorizationServers:   h.authorizationServers,
	}

	if err := json.NewEncoder(w).Encode(metadata); err != nil && h.logger.Enabled(r.Context(), slog.LevelInfo) {
		h.logger.LogAttrs(r.Context(), slog.LevelInfo, "Failed to write protected resource metadata response", slog.Any("error", err))
	}
}
