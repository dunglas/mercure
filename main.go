package main

import (
	"os"

	fluentd "github.com/joonix/log"
	log "github.com/sirupsen/logrus"

	_ "net/http/pprof"

	"github.com/dunglas/mercure/hub"
	_ "github.com/joho/godotenv/autoload"
)

func init() {
	switch os.Getenv("LOG_FORMAT") {
	case "JSON":
		log.SetFormatter(&log.JSONFormatter{})
		return
	case "FLUENTD":
		log.SetFormatter(fluentd.NewFormatter())
	}

	if os.Getenv("DEBUG") == "1" {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	hub, err := hub.NewHubFromEnv()
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if err = hub.Stop(); err != nil {
			log.Fatalln(err)
		}
	}()

	hub.Serve()
}
