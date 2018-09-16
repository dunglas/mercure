package main

import (
	"os"

	fluentd "github.com/joonix/log"
	log "github.com/sirupsen/logrus"

	"github.com/dunglas/mercure/hub"
	_ "github.com/joho/godotenv/autoload"
)

func init() {
	switch os.Getenv("LOG_FORMAT") {
	case "JSON":
		log.SetFormatter(&log.JSONFormatter{})
		return
	case "FLUENTD":
		log.SetFormatter(&fluentd.FluentdFormatter{})
	}
}

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
