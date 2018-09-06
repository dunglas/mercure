package main

import (
	"log"
	"os"

	"github.com/dunglas/mercure/hub"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	hub, err := hub.NewHubFromEnv()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	hub.Start()
	hub.Serve()
}
