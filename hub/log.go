package hub

import (
	"net/http"

	fluentd "github.com/joonix/log"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// TODO: delete me
func (h *Hub) createLogFields(r *http.Request, u *Update, s *Subscriber) log.Fields {
	return createBaseLogFields(h.config.GetBool("debug"), r.RemoteAddr, u, s)
}

// TODO: rename me
func createBaseLogFields(debug bool, remoteAddr string, u *Update, s *Subscriber) log.Fields {
	fields := log.Fields{
		"remote_addr": remoteAddr,
	}

	if u != nil {
		fields["event_id"] = u.ID
		fields["event_type"] = u.Type
		fields["event_retry"] = u.Retry
		fields["update_topics"] = u.Topics
		fields["update_targets"] = targetsMapToArray(u.Targets)

		if debug {
			fields["update_data"] = u.Data
		}

	}

	if s != nil {
		fields["last_event_id"] = s.LastEventID
		fields["subscriber_topics"] = s.Topics
		fields["subscriber_targets"] = targetsMapToArray(s.Targets)
	}

	return fields
}

func targetsMapToArray(t map[string]struct{}) []string {
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
