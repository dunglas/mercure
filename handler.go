package mercure

import (
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
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

	h.handler = secureMiddleware.Handler(
		cors.New(cors.Options{
			AllowedOrigins:   h.corsOrigins,
			AllowCredentials: true,
			AllowedHeaders:   []string{"authorization", "cache-control", "last-event-id"},
			Debug:            h.debug,
		}).Handler(router),
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

	// New 3-segment route (more specific, registered first).
	r.HandleFunc(subscriptionMatchURL, h.SubscriptionHandler).Methods(http.MethodGet)

	// The modern 2-segment collection route /subscriptions/{matchType}/{match}
	// and the legacy /subscriptions/{topic}/{subscriber} route have the same
	// shape. In modern-only mode only the modern route is registered, so there
	// is no ambiguity and no per-request check is added — this is the hot
	// path. When compat is enabled, a MatcherFunc restricts the modern route
	// to registered matcher types so unrelated paths fall through to legacy.
	if h.isBackwardCompatiblyEnabledWith(8) {
		r.HandleFunc(subscriptionsForMatchURL, h.SubscriptionsHandler).
			Methods(http.MethodGet).
			MatcherFunc(h.isRegisteredMatcherType)

		r.HandleFunc(subscriptionURL, h.SubscriptionHandler).Methods(http.MethodGet)
		r.HandleFunc(subscriptionsForTopicURL, h.SubscriptionsHandler).Methods(http.MethodGet)
	} else {
		r.HandleFunc(subscriptionsForMatchURL, h.SubscriptionsHandler).Methods(http.MethodGet)
	}

	r.HandleFunc(subscriptionsURL, h.SubscriptionsHandler).Methods(http.MethodGet)
}

// subscriptionsForMatchPrefixLen is the length of the path prefix up to and
// including the trailing slash before the {matchType} segment, used to
// disambiguate the modern 2-segment collection route from the legacy
// {topic}/{subscriber} route in isRegisteredMatcherType.
var subscriptionsForMatchPrefixLen = len(defaultHubURL + subscriptionsPath + "/") //nolint:gochecknoglobals

// isRegisteredMatcherType is a mux.MatcherFunc that accepts requests whose
// {matchType} path segment corresponds to a registered matcher type. Used
// to disambiguate the modern 2-segment collection route from the legacy
// {topic}/{subscriber} route when backward compatibility is enabled. The
// path matcher has already accepted the overall shape, so we only need to
// peel off the first segment after /subscriptions/.
func (h *Hub) isRegisteredMatcherType(r *http.Request, _ *mux.RouteMatch) bool {
	path := r.URL.EscapedPath()
	if len(path) <= subscriptionsForMatchPrefixLen {
		return false
	}

	rest := path[subscriptionsForMatchPrefixLen:]

	segment, _, found := strings.Cut(rest, "/")
	if !found {
		return false
	}

	mt, err := url.QueryUnescape(segment)
	if err != nil || mt == "" {
		return false
	}

	_, ok := h.topicSelectorStore.ResolveMatcherType(mt)

	return ok
}
