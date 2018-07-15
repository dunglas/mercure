package main

import (
	"log"
	"net/http"
	"os"

	"github.com/dunglas/mercure/hub"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	hub := hub.NewHub()
	hub.Start()

	http.Handle("/subscribe", http.HandlerFunc(hub.SubscribeHandler))
	http.Handle("/publish", http.HandlerFunc(hub.PublishHandler))
	http.Handle("/", http.FileServer(http.Dir("public")))

	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = ":80"
	}
	log.Printf("Mercure started on %s.\n", listen)

	http.ListenAndServe(listen, nil)
}
