package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/dunglas/mercure/hub"
	"github.com/gorilla/handlers"
	_ "github.com/joho/godotenv/autoload"
)

type options struct {
	debug              bool
	addr               string
	publisherJWTKey    []byte
	subscriberJWTKey   []byte
	corsAllowedOrigins []string
}

func parseEnv() (*options, error) {
	listen := os.Getenv("ADDR")
	if listen == "" {
		listen = ":80"
	}

	options := &options{
		os.Getenv("DEBUG") != "",
		listen,
		[]byte(os.Getenv("PUBLISHER_JWT_KEY")),
		[]byte(os.Getenv("SUBSCRIBER_JWT_KEY")),
		strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ","),
	}

	missingEnv := []interface{}{}
	if len(options.publisherJWTKey) == 0 {
		missingEnv = append(missingEnv, "PUBLISHER_JWT_KEY")
	}
	if len(options.subscriberJWTKey) == 0 {
		missingEnv = append(missingEnv, "SUBSCRIBER_JWT_KEY")
	}

	switch len(missingEnv) {
	case 1:
		return nil, fmt.Errorf("The \"%s\" environment variable must be defined", missingEnv[0])
	case 2:
		return nil, fmt.Errorf("The \"%s\" and \"%s\" environment variables must be defined", missingEnv...)
	}

	return options, nil
}

func main() {
	options, err := parseEnv()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	hub := hub.NewHub(options.publisherJWTKey, options.subscriberJWTKey)
	hub.Start()

	serve(options, hub)
}

func serve(options *options, hub *hub.Hub) {
	allowedOrigins := handlers.AllowedOrigins(options.corsAllowedOrigins)
	subscribeCORS := handlers.CORS(handlers.AllowCredentials(), allowedOrigins)

	http.Handle("/", http.FileServer(http.Dir("public")))
	http.Handle("/publish", http.HandlerFunc(hub.PublishHandler))
	http.Handle("/subscribe", subscribeCORS(http.HandlerFunc(hub.SubscribeHandler)))

	loggingHandler := handlers.CombinedLoggingHandler(os.Stderr, http.DefaultServeMux)
	recoveryHandler := handlers.RecoveryHandler(handlers.PrintRecoveryStack(options.debug))(loggingHandler)

	srv := &http.Server{Addr: options.addr, Handler: recoveryHandler}
	srv.RegisterOnShutdown(func() {
		hub.Stop()
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

	log.Printf("Mercure started on \"%s\"\n", options.addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Println(err)
	}

	<-idleConnsClosed
}
