package hub

import (
	fluentd "github.com/joonix/log"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func addUpdateFields(f log.Fields, u *Update, debug bool) log.Fields {
	f["event_id"] = u.ID
	f["event_type"] = u.Type
	f["event_retry"] = u.Retry
	f["update_topics"] = u.Topics
	f["update_targets"] = targetsMapToSlice(u.Targets)

	if debug {
		f["update_data"] = u.Data
	}

	return f
}

func createFields(u *Update, s *Subscriber) log.Fields {
	f := addUpdateFields(log.Fields{}, u, s.debug)
	for k, v := range s.logFields {
		f[k] = v
	}

	return f
}

func targetsMapToSlice(t map[string]struct{}) []string {
	targets := make([]string, len(t))

	var i int
	for target := range t {
		targets[i] = target
		i++
	}

	return targets
}

// InitLogrus configures the global logger.
func InitLogrus() {
	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	switch viper.GetString("log_format") {
	case "JSON":
		log.SetFormatter(&log.JSONFormatter{})
	case "FLUENTD":
		log.SetFormatter(fluentd.NewFormatter())
	}
}
