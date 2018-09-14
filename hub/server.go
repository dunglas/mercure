package hub

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/handlers"
	"github.com/unrolled/secure"
	"golang.org/x/crypto/acme/autocert"
)

// Serve starts the HTTP server
func (h *Hub) Serve() {
	srv := &http.Server{
		Addr:    h.options.Addr,
		Handler: h.chainHandlers(),
	}
	srv.RegisterOnShutdown(func() {
		h.Stop()
	})

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		if err := srv.Shutdown(context.Background()); err != nil {
			log.Println(err)
		}
		log.Println("My Baby Shot Me Down")
		close(idleConnsClosed)
	}()

	acme := len(h.options.AcmeHosts) > 0
	var err error

	if !acme && h.options.CertFile == "" && h.options.KeyFile == "" {
		log.Printf("Mercure is starting (http)...")
		err = srv.ListenAndServe()
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
			srv.TLSConfig = certManager.TLSConfig()

			// Mandatory for Let's Encrypt http-01 challenge
			go http.ListenAndServe(":http", certManager.HTTPHandler(nil))
		}

		log.Printf("Mercure is starting (https)...")
		err = srv.ListenAndServeTLS(h.options.CertFile, h.options.KeyFile)
	}

	if err != http.ErrServerClosed {
		log.Println(err)
	}

	<-idleConnsClosed
}

// chainHandlers configures and chains handlers
func (h *Hub) chainHandlers() http.Handler {
	if h.options.Demo {
		http.Handle("/", http.FileServer(http.Dir("public")))
	}
	http.Handle("/publish", http.HandlerFunc(h.PublishHandler))

	var s http.Handler
	if len(h.options.CorsAllowedOrigins) > 0 {
		allowedOrigins := handlers.AllowedOrigins(h.options.CorsAllowedOrigins)
		subscribeCORS := handlers.CORS(handlers.AllowCredentials(), allowedOrigins)

		s = subscribeCORS(http.HandlerFunc(h.SubscribeHandler))
	} else {
		s = http.HandlerFunc(h.SubscribeHandler)
	}
	http.Handle("/subscribe", s)

	secureMiddleware := secure.New(secure.Options{
		IsDevelopment:         h.options.Debug,
		AllowedHosts:          h.options.AcmeHosts,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self'",
	})

	secureHandler := secureMiddleware.Handler(http.DefaultServeMux)
	loggingHandler := handlers.CombinedLoggingHandler(os.Stderr, secureHandler)
	recoveryHandler := handlers.RecoveryHandler(handlers.PrintRecoveryStack(h.options.Debug))(loggingHandler)

	return recoveryHandler
}
