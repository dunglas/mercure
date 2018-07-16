package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dunglas/mercure/hub"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	publisherJwtKey := os.Getenv("PUBLISHER_JWT_KEY")
	if publisherJwtKey == "" {
		log.Panicln("The \"PUBLISHER_JWT_KEY\" environment variable is not set.")
	}

	subscriberJwtKey := os.Getenv("SUBSCRIBER_JWT_KEY")
	if subscriberJwtKey == "" {
		log.Panicln("The \"SUBSCRIBER_JWT_KEY\" environment variable is not set, authorization support is disabled.")
	}

	hub := hub.NewHub([]byte(publisherJwtKey), []byte(subscriberJwtKey))
	hub.Start()

	http.Handle("/", http.FileServer(http.Dir("public")))
	http.Handle("/publish", http.HandlerFunc(hub.PublishHandler))
	http.Handle("/subscribe", http.HandlerFunc(hub.SubscribeHandler))

	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = ":80"
	}
	log.Printf("Mercure started on %s.\n", listen)

	http.ListenAndServe(listen, nil)
}
