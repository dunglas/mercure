package mercure

import (
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/unrolled/secure"
)

const (
	defaultHubURL  = "/.well-known/mercure"
	defaultUIURL   = defaultHubURL + "/ui/"
	defaultDemoURL = defaultUIURL + "demo/"
)

func (h *Hub) initHandler() {
	router := mux.NewRouter()
	router.UseEncodedPath()
	router.SkipClean(true)

	csp := "default-src 'self'"

	if h.demo {
		router.PathPrefix(defaultDemoURL).HandlerFunc(h.Demo).Methods(http.MethodGet, http.MethodHead)
	}

	if h.ui {
		public, err := fs.Sub(uiContent, "public")
		if err != nil {
			panic(err)
		}

		router.PathPrefix(defaultUIURL).Handler(http.StripPrefix(defaultUIURL, http.FileServer(http.FS(public))))

		csp += " mercure.rocks cdn.jsdelivr.net"
	}

	h.registerSubscriptionHandlers(router)

	if h.subscriberConfigured || h.anonymous {
		router.HandleFunc(defaultHubURL, h.SubscribeHandler).Methods(http.MethodGet, http.MethodHead, methodQuery)
	}

	if h.publisherConfigured {
		router.HandleFunc(defaultHubURL, h.PublishHandler).Methods(http.MethodPost)
	}

	// Advertise OAuth 2.0 protected resource metadata (RFC 9728) only when the
	// hub validates access tokens; a pure-anonymous hub is not a protected
	// resource.
	if h.publisherConfigured || h.subscriberConfigured {
		router.HandleFunc(protectedResourceMetadataPath, h.ProtectedResourceMetadataHandler).Methods(http.MethodGet, http.MethodHead)
	}

	secureMiddleware := secure.New(secure.Options{
		IsDevelopment:         h.debug,
		AllowedHosts:          h.allowedHosts,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: csp,
	})

	h.handler = secureMiddleware.Handler(h.corsHandler(router))
}

// corsHandler wraps the router with CORS when origins are configured,
// otherwise returns it unchanged.
func (h *Hub) corsHandler(router http.Handler) http.Handler {
	if len(h.corsOrigins) == 0 {
		return router
	}

	// The protocol forbids combining a wildcard Access-Control-Allow-Origin
	// with credentials: cookies cross origins only when the allowed origins
	// form an explicit allowlist. With "*", credentialed responses are
	// rejected by browsers anyway, so disable credentials instead of shipping
	// a header pair that can never work.
	allowCredentials := !slices.Contains(h.corsOrigins, "*")

	return cors.New(cors.Options{
		AllowedOrigins:   h.corsOrigins,
		AllowCredentials: allowCredentials,
		AllowedMethods:   []string{http.MethodGet, http.MethodHead, http.MethodPost, methodQuery},
		AllowedHeaders:   []string{authorizationHeader, "cache-control", "last-event-id"},
		// Exposed so cross-origin subscribers can read the subscription API's
		// rel="mercure" Link header, which carries the last-event-id cursor.
		ExposedHeaders: []string{"Link"},
		Debug:          h.debug,
	}).Handler(router)
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Reject a request whose origin is not in the public-URL allowlist before
	// deriving any identity from it (see requestIdentity). The origin is the one
	// an embedding server resolved (the Caddy module, from Caddy's trusted
	// placeholders), else the request's own scheme and Host.
	if len(h.allowedOrigins) > 0 {
		scheme, host := h.requestOrigin(r)
		if !slices.Contains(h.allowedOrigins, strings.ToLower(scheme+"://"+host)) {
			http.Error(w, http.StatusText(http.StatusMisdirectedRequest), http.StatusMisdirectedRequest)

			return
		}
	}

	h.handler.ServeHTTP(w, r)
}

func (h *Hub) registerSubscriptionHandlers(r *mux.Router) {
	if !h.subscriptions {
		return
	}

	if _, ok := h.transport.(TransportSubscribers); !ok {
		if h.logger.Enabled(h.ctx, slog.LevelError) {
			h.logger.LogAttrs(h.ctx, slog.LevelError, "The current transport doesn't support subscriptions. Subscription API disabled.")
		}

		return
	}

	r.UseEncodedPath()
	r.SkipClean(true)

	// 3-segment route (more specific, registered first).
	r.HandleFunc(subscriptionMatchURL, h.SubscriptionHandler).Methods(http.MethodGet)

	// The collection route /subscriptions/{match_type}/{match} and the
	// deprecated /subscriptions/{topic}/{subscriber} route have the same
	// shape. In modern-only mode only the modern route is registered, so
	// there is no ambiguity and no per-request check is added — this is the
	// hot path. Under the deprecated_topic tag and compatibility mode, the
	// deprecated registration guards the modern route with a MatcherFunc and
	// adds the v8 routes.
	if !h.registerDeprecatedSubscriptionHandlers(r) {
		r.HandleFunc(subscriptionsForMatchURL, h.SubscriptionsHandler).Methods(http.MethodGet)
	}

	r.HandleFunc(subscriptionsURL, h.SubscriptionsHandler).Methods(http.MethodGet)
}
