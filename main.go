package main

import (
	"github.com/dunglas/mercure/config"
	fluentd "github.com/joonix/log"
	log "github.com/sirupsen/logrus"

	_ "net/http/pprof"

	_ "github.com/dunglas/mercure/config"
	"github.com/dunglas/mercure/hub"
)

func init() {
	switch config.GetString("LOG_FORMAT") {
	case "JSON":
		log.SetFormatter(&log.JSONFormatter{})
		return
	case "FLUENTD":
		log.SetFormatter(fluentd.NewFormatter())
	}

	if config.GetString("DEBUG") == "1" {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	hub, db, err := hub.NewHubFromEnv()
	if err != nil {
		log.Fatalln(err)
	}

	defer db.Close()

	hub.Start()
	hub.Serve()
}
