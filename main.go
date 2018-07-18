package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dunglas/mercure/hub"
	"github.com/gorilla/handlers"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	debug := os.Getenv("DEBUG") != ""

	publisherJWTKey := os.Getenv("PUBLISHER_JWT_KEY")
	if publisherJWTKey == "" {
		log.Panicln("The \"PUBLISHER_JWT_KEY\" environment variable is not set.")
	}

	subscriberJWTKey := os.Getenv("SUBSCRIBER_JWT_KEY")
	if subscriberJWTKey == "" {
		log.Panicln("The \"SUBSCRIBER_JWT_KEY\" environment variable is not set, authorization support is disabled.")
	}

	hub := hub.NewHub([]byte(publisherJWTKey), []byte(subscriberJWTKey))
	hub.Start()

	allowedOrigins := handlers.AllowedOrigins(strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ","))
	subscribeCORS := handlers.CORS(handlers.AllowCredentials(), allowedOrigins)

	http.Handle("/", http.FileServer(http.Dir("public")))
	http.Handle("/publish", http.HandlerFunc(hub.PublishHandler))
	http.Handle("/subscribe", subscribeCORS(http.HandlerFunc(hub.SubscribeHandler)))

	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = ":80"
	}
	log.Printf("Mercure started on %s.\n", listen)

	loggingHandler := handlers.CombinedLoggingHandler(os.Stderr, http.DefaultServeMux)
	recoveryHandler := handlers.RecoveryHandler(handlers.PrintRecoveryStack(debug))(loggingHandler)

	http.ListenAndServe(listen, recoveryHandler)
}
