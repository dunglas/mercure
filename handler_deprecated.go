//go:build deprecated_server

package mercure

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/unrolled/secure"
	"golang.org/x/crypto/acme/autocert"
)

// Serve starts the HTTP server.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func (h *Hub) Serve() { //nolint:funlen
	addr := h.config.GetString("addr")

	h.server = &http.Server{
		Addr:              addr,
		Handler:           h.baseHandler(),
		ReadTimeout:       h.config.GetDuration("read_timeout"),
		ReadHeaderTimeout: h.config.GetDuration("read_header_timeout"),
		WriteTimeout:      h.config.GetDuration("write_timeout"),
	}

	if _, ok := h.metrics.(*PrometheusMetrics); ok {
		addr := h.config.GetString("metrics_addr")

		h.metricsServer = &http.Server{
			Addr:              addr,
			Handler:           h.metricsHandler(),
			ReadTimeout:       h.config.GetDuration("read_timeout"),
			ReadHeaderTimeout: h.config.GetDuration("read_header_timeout"),
			WriteTimeout:      h.config.GetDuration("write_timeout"),
		}

		h.logger.Info("Mercure metrics started", slog.String("addr", addr))

		go h.metricsServer.ListenAndServe()
	}

	acme := len(h.allowedHosts) > 0

	certFile := h.config.GetString("cert_file")
	keyFile := h.config.GetString("key_file")

	done := h.listenShutdown()

	var err error

	if !acme && certFile == "" && keyFile == "" { //nolint:nestif
		h.logger.Info("Mercure started", slog.String("protocol", "http"), slog.String("addr", addr))

		err = h.server.ListenAndServe()
	} else {
		// TLS
		if acme {
			certManager := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(h.allowedHosts...),
			}

			acmeCertDir := h.config.GetString("acme_cert_dir")
			if acmeCertDir != "" {
				certManager.Cache = autocert.DirCache(acmeCertDir)
			}

			h.server.TLSConfig = certManager.TLSConfig()

			// Mandatory for Let's Encrypt http-01 challenge
			go http.ListenAndServe(h.config.GetString("acme_http01_addr"), certManager.HTTPHandler(nil)) //nolint:gosec
		}

		h.logger.Info("Mercure started", slog.String("protocol", "https"), slog.String("addr", addr))

		err = h.server.ListenAndServeTLS(certFile, keyFile)
	}

	if !errors.Is(err, http.ErrServerClosed) {
		h.logger.Error("Unexpected error", slog.Any("error", err))
	}

	<-done
}

// Deprecated: use the Caddy server module or the standalone library instead.
func (h *Hub) listenShutdown() <-chan struct{} {
	idleConnsClosed := make(chan struct{})

	h.server.RegisterOnShutdown(func() {
		select {
		case <-idleConnsClosed:
		default:
			close(idleConnsClosed)
		}
	})

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		if err := h.server.Shutdown(context.Background()); err != nil {
			h.logger.Error("Unexpected error during server shutdown", slog.Any("error", err))
		}

		if h.metricsServer != nil {
			if err := h.metricsServer.Shutdown(context.Background()); err != nil {
				h.logger.Error("Unexpected error during metrics server shutdown", slog.Any("error", err))
			}
		}

		h.logger.Info("My Baby Shot Me Down")

		select {
		case <-idleConnsClosed:
		default:
			close(idleConnsClosed)
		}
	}()

	return idleConnsClosed
}

// chainHandlers configures and chains handlers.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func (h *Hub) chainHandlers() http.Handler { //nolint:funlen
	r := mux.NewRouter()
	h.registerSubscriptionHandlers(context.Background(), r)

	r.HandleFunc(defaultHubURL, h.SubscribeHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc(defaultHubURL, h.PublishHandler).Methods(http.MethodPost)

	csp := "default-src 'self'"

	if h.demo {
		r.PathPrefix("/demo").HandlerFunc(h.Demo).Methods(http.MethodGet, http.MethodHead)
	}

	if h.ui {
		public, err := fs.Sub(uiContent, "public")
		if err != nil {
			panic(err)
		}

		r.PathPrefix("/").Handler(http.FileServer(http.FS(public)))

		csp += " mercure.rocks cdn.jsdelivr.net"
	} else {
		r.HandleFunc("/", welcomeHandler).Methods(http.MethodGet, http.MethodHead)
	}

	secureMiddleware := secure.New(secure.Options{
		IsDevelopment:         h.debug,
		AllowedHosts:          h.allowedHosts,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: csp,
	})

	var corsHandler http.Handler

	if len(h.corsOrigins) > 0 {
		corsHandler = cors.New(cors.Options{
			AllowedOrigins:   h.corsOrigins,
			AllowCredentials: true,
			AllowedHeaders:   []string{"authorization", "cache-control", "last-event-id"},
		}).Handler(r)
	} else {
		corsHandler = r
	}

	var compressHandler http.Handler
	if h.config.GetBool("compress") {
		compressHandler = handlers.CompressHandler(corsHandler)
	} else {
		compressHandler = corsHandler
	}

	var useForwardedHeadersHandlers http.Handler
	if h.config.GetBool("use_forwarded_headers") {
		useForwardedHeadersHandlers = handlers.ProxyHeaders(compressHandler)
	} else {
		useForwardedHeadersHandlers = compressHandler
	}

	secureHandler := secureMiddleware.Handler(useForwardedHeadersHandlers)

	var loggingHandler http.Handler

	if h.logger.Enabled(context.Background(), slog.LevelError) {
		loggingHandler = handlers.CombinedLoggingHandler(os.Stderr, secureHandler)
	} else {
		loggingHandler = secureHandler
	}

	recoveryHandler := handlers.RecoveryHandler(
		handlers.RecoveryLogger(slogRecoveryHandlerLogger{h.logger}),
		handlers.PrintRecoveryStack(h.debug),
	)(loggingHandler)

	return recoveryHandler
}

// Deprecated: use the Caddy server module or the standalone library instead.
func (h *Hub) baseHandler() http.Handler {
	mainRouter := mux.NewRouter()
	mainRouter.UseEncodedPath()
	mainRouter.SkipClean(true)

	// Register /healthz (if enabled, in a way that doesn't pollute the HTTP logs).
	registerHealthz(mainRouter)

	handler := h.chainHandlers()
	mainRouter.PathPrefix("/").Handler(handler)

	return mainRouter
}

// Deprecated: use the Caddy server module or the standalone library instead.
func (h *Hub) metricsHandler() http.Handler {
	router := mux.NewRouter()

	registerHealthz(router)
	h.metrics.(*PrometheusMetrics).Register(router.PathPrefix("/").Subrouter())

	return router
}

// Deprecated: use the Caddy server module or the standalone library instead.
func registerHealthz(router *mux.Router) {
	router.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	}).Methods(http.MethodGet, http.MethodHead)
}

// Deprecated: use the Caddy server module or the standalone library instead.
func welcomeHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, `<!DOCTYPE html>
<title>Mercure Hub</title>
<h1>Welcome to <a href="https://mercure.rocks">Mercure</a>!</h1>`)
}

// Deprecated: use the Caddy server module or the standalone library instead.
type slogRecoveryHandlerLogger struct {
	logger *slog.Logger
}

func (l slogRecoveryHandlerLogger) Println(args ...any) {
	l.logger.Error(fmt.Sprint(args...))
}
