package mercure

import (
	"io/fs"
	"log/slog"
	"net/http"

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

		csp += " mercure.rocks cdn.jsdelivr.net cdnjs.cloudflare.com fonts.googleapis.com; script-src 'self' cdn.jsdelivr.net cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' cdn.jsdelivr.net cdnjs.cloudflare.com fonts.googleapis.com; font-src 'self' fonts.gstatic.com cdnjs.cloudflare.com data:; connect-src 'self' cdn.jsdelivr.net"
	}

	h.registerSubscriptionHandlers(router)

	if h.subscriberJWTKeyFunc != nil || h.anonymous {
		router.HandleFunc(defaultHubURL, h.SubscribeHandler).Methods(http.MethodGet, http.MethodHead)
	}

	if h.publisherJWTKeyFunc != nil {
		router.HandleFunc(defaultHubURL, h.PublishHandler).Methods(http.MethodPost)
	}

	secureMiddleware := secure.New(secure.Options{
		IsDevelopment:         h.debug,
		AllowedHosts:          h.allowedHosts,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: csp,
	})

	if len(h.corsOrigins) == 0 {
		h.handler = secureMiddleware.Handler(router)

		return
	}

	corsOptions := cors.Options{
		AllowedOrigins:   h.corsOrigins,
		AllowCredentials: true,
		AllowedHeaders:   []string{"authorization", "cache-control", "last-event-id"},
		Debug:            h.debug,
	}

	if h.demo {
		// Expose Link header so cross-origin JS can read it for Mercure discovery.
		// Needed when UI runs on a separate origin (e.g., GoLand dev server with hot reload).
		corsOptions.ExposedHeaders = []string{"link"}
	}

	h.handler = secureMiddleware.Handler(
		cors.New(corsOptions).Handler(router),
	)
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	r.HandleFunc(subscriptionURL, h.SubscriptionHandler).Methods(http.MethodGet)
	r.HandleFunc(subscriptionsForTopicURL, h.SubscriptionsHandler).Methods(http.MethodGet)
	r.HandleFunc(subscriptionsURL, h.SubscriptionsHandler).Methods(http.MethodGet)
}
