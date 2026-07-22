package mercure

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// protectedResourceMetadataPath is the RFC 9728 well-known location for the
// hub's OAuth 2.0 protected resource metadata, derived from the hub URL.
const protectedResourceMetadataPath = "/.well-known/oauth-protected-resource" + defaultHubURL

// ProtectedResourceMetadataPath is the RFC 9728 well-known path the hub serves
// its protected resource metadata at. It is exported so an embedding server
// (for example, the Caddy module) can route to it without re-deriving the path.
const ProtectedResourceMetadataPath = protectedResourceMetadataPath

// protectedResourceMetadata is the subset of OAuth 2.0 Protected Resource
// Metadata (RFC 9728) the hub advertises. jwks_uri is intentionally omitted:
// the hub hosts no JWKS endpoint, and a single jwks_uri cannot represent the
// separate publisher and subscriber key sets.
type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	// AuthorizationDetailsTypesSupported advertises the RFC 9396
	// authorization detail types the hub understands.
	AuthorizationDetailsTypesSupported []string `json:"authorization_details_types_supported,omitempty"`
	// MercureCookie is the name of the cookie in which the hub accepts the
	// access token, a Mercure extension to RFC 6750. A browser client, which
	// cannot set an Authorization header, presents the token by setting a
	// cookie of this name. It is a dedicated member because bearer_methods_supported
	// values are constrained to the RFC 6750 methods (a cookie is not one of them).
	MercureCookie string `json:"mercure_cookie,omitempty"`
	// MercureSubscriptions advertises the active subscriptions feature (a
	// Mercure extension to RFC 9728) when the hub implements it.
	MercureSubscriptions bool `json:"mercure_subscriptions,omitempty"`
}

// bearerMethodsSupported lists the RFC 6750 token presentation methods the hub
// accepts. "query" is not accepted (RFC 9700 §4.3.2), and the cookie mechanism
// is not an RFC 6750 method, so it is advertised through the dedicated
// "mercure_cookie" member instead.
//
//nolint:gochecknoglobals
var bearerMethodsSupported = []string{"header"}

// ProtectedResourceMetadataHandler serves the hub's RFC 9728 protected
// resource metadata document.
func (h *Hub) ProtectedResourceMetadataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	identifier, _ := h.requestIdentity(r)

	metadata := protectedResourceMetadata{
		Resource:                           identifier,
		BearerMethodsSupported:             bearerMethodsSupported,
		AuthorizationServers:               h.authorizationServers,
		AuthorizationDetailsTypesSupported: []string{authorizationDetailTypeMercure},
		// The hub always accepts the access token in a cookie when it
		// validates tokens (this handler is only served in that case);
		// advertise the configured cookie name.
		MercureCookie:        h.cookieName,
		MercureSubscriptions: h.subscriptions,
	}

	if err := json.NewEncoder(w).Encode(metadata); err != nil && h.logger.Enabled(r.Context(), slog.LevelInfo) {
		h.logger.LogAttrs(r.Context(), slog.LevelInfo, "Failed to write protected resource metadata response", slog.Any("error", err))
	}
}
