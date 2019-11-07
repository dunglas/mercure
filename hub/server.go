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

const defaultHubURL = "/.well-known/mercure"

// Serve starts the HTTP server
func (h *Hub) Serve() {
	addr := h.config.GetString("addr")
	acmeHosts := h.config.GetStringSlice("acme_hosts")

	h.server = &http.Server{
		Addr:         addr,
		Handler:      h.chainHandlers(acmeHosts),
		ReadTimeout:  h.config.GetDuration("read_timeout"),
		WriteTimeout: h.config.GetDuration("write_timeout"),
	}

	acme := len(acmeHosts) > 0
	certFile := h.config.GetString("cert_file")
	keyFile := h.config.GetString("key_file")

	done := h.listenShutdown()
	var err error

	if !acme && certFile == "" && keyFile == "" {
		log.WithFields(log.Fields{"protocol": "http", "addr": addr}).Info("Mercure started")
		err = h.server.ListenAndServe()
	} else {
		// TLS
		if acme {
			certManager := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(acmeHosts...),
			}

			acmeCertDir := h.config.GetString("acme_cert_dir")
			if acmeCertDir != "" {
				certManager.Cache = autocert.DirCache(acmeCertDir)
			}
			h.server.TLSConfig = certManager.TLSConfig()

			// Mandatory for Let's Encrypt http-01 challenge
			go http.ListenAndServe(h.config.GetString("acme_http01_addr"), certManager.HTTPHandler(nil))
		}

		log.WithFields(log.Fields{"protocol": "https", "addr": addr}).Info("Mercure started")
		err = h.server.ListenAndServeTLS(certFile, keyFile)
	}

	if err != http.ErrServerClosed {
		log.Fatal(err)
	}

	<-done
}

func (h *Hub) listenShutdown() chan struct{} {
	idleConnsClosed := make(chan struct{})

	h.server.RegisterOnShutdown(func() {
		h.Stop()
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
			log.Error(err)
		}
		log.Infoln("My Baby Shot Me Down")
		select {
		case <-idleConnsClosed:
		default:
			close(idleConnsClosed)
		}
	}()

	return idleConnsClosed
}

// chainHandlers configures and chains handlers
func (h *Hub) chainHandlers(acmeHosts []string) http.Handler {
	debug := h.config.GetBool("debug")

	r := mux.NewRouter()

	r.HandleFunc(defaultHubURL, h.SubscribeHandler).Methods("GET", "HEAD")
	r.HandleFunc(defaultHubURL, h.PublishHandler).Methods("POST")
	if debug || h.config.GetBool("demo") {
		r.PathPrefix("/demo").HandlerFunc(Demo).Methods("GET", "HEAD")
		r.PathPrefix("/").Handler(http.FileServer(http.Dir("public")))
	} else {
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<!DOCTYPE html>
<title>Mercure Hub</title>
<h1>Welcome to <a href="https://mercure.rocks">Mercure</a>!</h1>`)
		}).Methods("GET", "HEAD")
	}

	secureMiddleware := secure.New(secure.Options{
		IsDevelopment:         debug,
		AllowedHosts:          acmeHosts,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self'",
	})

	var corsHandler http.Handler
	corsAllowedOrigins := h.config.GetStringSlice("cors_allowed_origins")
	if len(corsAllowedOrigins) > 0 {
		allowedOrigins := handlers.AllowedOrigins(corsAllowedOrigins)
		allowedHeaders := handlers.AllowedHeaders([]string{"authorization", "cache-control"})

		corsHandler = handlers.CORS(handlers.AllowCredentials(), allowedOrigins, allowedHeaders)(r)
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
	loggingHandler := handlers.CombinedLoggingHandler(os.Stderr, secureHandler)
	recoveryHandler := handlers.RecoveryHandler(
		handlers.RecoveryLogger(log.New()),
		handlers.PrintRecoveryStack(debug),
	)(loggingHandler)

	return recoveryHandler
}
