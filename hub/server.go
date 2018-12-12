package hub

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/unrolled/secure"
	"golang.org/x/crypto/acme/autocert"
)

// Serve starts the HTTP server
func (h *Hub) Serve() {
	h.server = &http.Server{
		Addr:    h.options.Addr,
		Handler: h.chainHandlers(),
	}
	h.server.RegisterOnShutdown(func() {
		h.Stop()
	})

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		if err := h.server.Shutdown(context.Background()); err != nil {
			log.Error(err)
		}
		log.Infoln("My Baby Shot Me Down")
		close(idleConnsClosed)
	}()

	acme := len(h.options.AcmeHosts) > 0
	var err error

	if !acme && h.options.CertFile == "" && h.options.KeyFile == "" {
		log.WithFields(log.Fields{"protocol": "http"}).Info("Mercure started")
		err = h.server.ListenAndServe()
	} else {
		// TLS
		if acme {
			certManager := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(h.options.AcmeHosts...),
			}
			if h.options.AcmeCertDir != "" {
				certManager.Cache = autocert.DirCache(h.options.AcmeCertDir)
			}
			h.server.TLSConfig = certManager.TLSConfig()

			// Mandatory for Let's Encrypt http-01 challenge
			go http.ListenAndServe(":http", certManager.HTTPHandler(nil))
		}

		log.WithFields(log.Fields{"protocol": "https"}).Info("Mercure started")
		err = h.server.ListenAndServeTLS(h.options.CertFile, h.options.KeyFile)
	}

	if err != http.ErrServerClosed {
		log.Error(err)
	}

	<-idleConnsClosed
}

// chainHandlers configures and chains handlers
func (h *Hub) chainHandlers() http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/hub", h.SubscribeHandler).Methods("GET", "HEAD")
	r.HandleFunc("/hub", h.PublishHandler).Methods("POST")
	if h.options.Demo {
		r.PathPrefix("/demo").HandlerFunc(demo)
		r.PathPrefix("/").Handler(http.FileServer(http.Dir("public")))
	} else {
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<!DOCTYPE html>
<title>Mercure Hub</title>
<h1>Welcome to <a href="https://mercure.rocks">Mercure</a>!</h1>`)
		})
	}

	secureMiddleware := secure.New(secure.Options{
		IsDevelopment:         h.options.Debug,
		AllowedHosts:          h.options.AcmeHosts,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self'",
	})

	var corsHandler http.Handler
	if len(h.options.CorsAllowedOrigins) > 0 {
		allowedOrigins := handlers.AllowedOrigins(h.options.CorsAllowedOrigins)
		allowedHeaders := handlers.AllowedHeaders([]string{"authorization"})

		corsHandler = handlers.CORS(handlers.AllowCredentials(), allowedOrigins, allowedHeaders)(r)
	} else {
		corsHandler = r
	}

	compressHandler := handlers.CompressHandler(corsHandler)
	secureHandler := secureMiddleware.Handler(compressHandler)
	loggingHandler := handlers.CombinedLoggingHandler(os.Stderr, secureHandler)
	recoveryHandler := handlers.RecoveryHandler(
		handlers.RecoveryLogger(log.New()),
		handlers.PrintRecoveryStack(h.options.Debug),
	)(loggingHandler)

	return recoveryHandler
}
