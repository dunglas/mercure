package main

import (
	"log"

	"github.com/dunglas/mercure/hub"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	db, err := hub.NewBoltFromEnv()
	exitOnError(err)
	defer db.Close()

	hub, err := hub.NewHubFromEnv(&hub.BoltHistory{DB: db})
	exitOnError(err)

	hub.Start()
	hub.Serve()
}

func exitOnError(err error) {
	if err == nil {
		return
	}

	log.Fatal(err)
}
