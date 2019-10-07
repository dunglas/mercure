package hub

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

func (h *Hub) createLogFields(r *http.Request, u *Update, s *Subscriber) log.Fields {
	fields := log.Fields{
		"remote_addr":    r.RemoteAddr,
		"event_id":       u.ID,
		"event_type":     u.Type,
		"event_retry":    u.Retry,
		"update_topics":  u.Topics,
		"update_targets": targetsMapToArray(u.Targets),
	}
	if h.options.Debug {
		fields["update_data"] = u.Data
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
