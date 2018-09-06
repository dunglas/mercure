package hub

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/handlers"
)

// Serve starts the HTTP server
func (h *Hub) Serve() {
	allowedOrigins := handlers.AllowedOrigins(h.options.CorsAllowedOrigins)
	subscribeCORS := handlers.CORS(handlers.AllowCredentials(), allowedOrigins)

	if h.options.Demo {
		http.Handle("/", http.FileServer(http.Dir("public")))
	}
	http.Handle("/publish", http.HandlerFunc(h.PublishHandler))
	http.Handle("/subscribe", subscribeCORS(http.HandlerFunc(h.SubscribeHandler)))

	loggingHandler := handlers.CombinedLoggingHandler(os.Stderr, http.DefaultServeMux)
	recoveryHandler := handlers.RecoveryHandler(handlers.PrintRecoveryStack(h.options.Debug))(loggingHandler)

	srv := &http.Server{Addr: h.options.Addr, Handler: recoveryHandler}
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

	log.Printf("Mercure started on %s", h.options.Addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Println(err)
	}

	<-idleConnsClosed
}
