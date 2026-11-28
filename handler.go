package mercure

import (
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rs/cors"
	"golang.org/x/net/publicsuffix"
)

const (
	defaultHubURL   = "/.well-known/mercure"
	defaultDebugURL = defaultHubURL + "/debug/"
)

// PlaygroundURLPrefix is the root path the insecure playground echo endpoints are
// served under (only when the playground is enabled). It sits outside the reserved
// /.well-known/mercure namespace, so the resources it serves are valid, subscribable
// Mercure topics; nesting them under the hub path would make them reserved.
//
// INSECURE + EXPERIMENTAL. Not covered by the backward compatibility promise.
const PlaygroundURLPrefix = "/playground/"

func (h *Hub) initHandler() {
	router := http.NewServeMux()

	csp := "default-src 'self'"

	if h.playground {
		router.HandleFunc("GET "+PlaygroundURLPrefix, h.Playground)
	}

	if h.debugger {
		router.HandleFunc("GET "+defaultDebugURL+"config.json", h.DebuggerConfigHandler)

		if h.playground && h.playgroundTokenFunc != nil {
			router.HandleFunc("GET "+defaultDebugURL+"playground-token", h.PlaygroundTokenHandler)
		}

		public, err := fs.Sub(debuggerContent, "public")
		if err != nil {
			panic(err)
		}

		router.Handle(defaultDebugURL, http.StripPrefix(defaultDebugURL, http.FileServer(http.FS(public))))

		// The page handles access tokens: everything it loads is vendored, with no inline script or style.
		csp = "default-src 'self'"
	}

	h.registerSubscriptionHandlers(router)

	if h.subscriberConfigured || h.anonymous {
		router.HandleFunc("GET "+defaultHubURL, h.SubscribeHandler)
		router.HandleFunc(methodQuery+" "+defaultHubURL, h.SubscribeHandler)
	}

	if h.publisherConfigured {
		router.HandleFunc("POST "+defaultHubURL, h.PublishHandler)
	}

	// Advertise OAuth 2.0 protected resource metadata (RFC 9728) only when the
	// hub validates access tokens; a pure-anonymous hub is not a protected
	// resource.
	if h.publisherConfigured || h.subscriberConfigured {
		router.HandleFunc("GET "+protectedResourceMetadataPath, h.ProtectedResourceMetadataHandler)
	}

	h.handler = securityHeaders(h.corsHandler(router), csp)
}

func securityHeaders(next http.Handler, csp string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Content-Security-Policy", csp)

		next.ServeHTTP(w, r)
	})
}

func spansArbitraryOrigins(origin string) bool {
	// Any site can send Origin: null from a sandboxed iframe or a data: URL.
	if origin == "null" {
		return true
	}

	prefix, suffix, found := strings.Cut(origin, "*")
	if !found {
		return false
	}

	if !strings.HasSuffix(prefix, "://") || !strings.HasPrefix(suffix, ".") || strings.Contains(suffix, "*") {
		return true
	}

	u, err := url.Parse(strings.ToLower(prefix + strings.TrimPrefix(suffix, ".")))
	if err != nil {
		return true
	}

	_, err = publicsuffix.EffectiveTLDPlusOne(u.Hostname())

	return err != nil
}

// corsHandler wraps the router with CORS when origins are configured,
// otherwise returns it unchanged.
func (h *Hub) corsHandler(router http.Handler) http.Handler {
	if len(h.corsOrigins) == 0 {
		return router
	}

	// Reflected wildcard origins must not expose credentials across registrable domains.
	allowCredentials := !slices.ContainsFunc(h.corsOrigins, spansArbitraryOrigins)

	return cors.New(cors.Options{
		AllowedOrigins:   h.corsOrigins,
		AllowCredentials: allowCredentials,
		AllowedMethods:   []string{http.MethodGet, http.MethodHead, http.MethodPost, methodQuery},
		AllowedHeaders:   []string{authorizationHeader, "cache-control", "last-event-id"},
		// Exposed so cross-origin subscribers can read the subscription API's
		// rel="mercure" Link header, which carries the last-event-id cursor,
		// and the Mercure-Last-Event-Id field a subscription answers with:
		// without it, a fetch-based cross-origin subscriber cannot detect
		// data loss when resuming.
		ExposedHeaders: []string{"Link", "Mercure-Last-Event-Id", "Accept-Query"},
		Debug:          h.debug,
	}).Handler(router)
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Reject a request whose origin is not in the public-URL allowlist before
	// deriving any identity from it (see requestIdentity). The origin is the one
	// an embedding server resolved (the Caddy module, from Caddy's request
	// placeholders), else the request's own scheme and Host.
	scheme, host := h.requestOrigin(r)

	// No Host (HTTP/1.0, or HTTP/2 without :authority) leaves a hub with no
	// configured identifier nothing to derive an identity from: no audience to
	// check tokens against, no valid RFC 9728 metadata to serve.
	if host == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}

	if len(h.allowedOrigins) > 0 {
		if !slices.Contains(h.allowedOrigins, strings.ToLower(scheme+"://"+host)) {
			http.Error(w, http.StatusText(http.StatusMisdirectedRequest), http.StatusMisdirectedRequest)

			return
		}
	}

	h.handler.ServeHTTP(w, r)
}

func (h *Hub) registerSubscriptionHandlers(router *http.ServeMux) {
	if !h.subscriptions {
		return
	}

	// API clients must be authorized in every protocol mode.
	if !h.subscriberConfigured {
		if h.logger.Enabled(h.ctx, slog.LevelError) {
			h.logger.LogAttrs(h.ctx, slog.LevelError, "No subscriber verifier is configured. Subscription API disabled.")
		}

		return
	}

	if _, ok := h.transport.(TransportSubscribers); !ok {
		if h.logger.Enabled(h.ctx, slog.LevelError) {
			h.logger.LogAttrs(h.ctx, slog.LevelError, "The current transport doesn't support subscriptions. Subscription API disabled.")
		}

		return
	}

	router.HandleFunc("GET "+subscriptionMatchURL, h.SubscriptionHandler)

	// Compatibility mode serves the same-shaped v8 /{topic}/{subscriber} from
	// this route; modern-only mode adds no per-request check.
	if !h.registerDeprecatedSubscriptionHandlers(router) {
		router.HandleFunc("GET "+subscriptionsForMatchURL, h.SubscriptionsHandler)
	}

	router.HandleFunc("GET "+subscriptionsURL, h.SubscriptionsHandler)
}
