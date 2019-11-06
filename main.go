package main

import (
	fluentd "github.com/joonix/log"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/dunglas/mercure/hub"
)

func init() {
	switch viper.GetString("log_format") {
	case "JSON":
		log.SetFormatter(&log.JSONFormatter{})
		return
	case "FLUENTD":
		log.SetFormatter(fluentd.NewFormatter())
	}

	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	hub, err := hub.NewHubFromConfig()
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
