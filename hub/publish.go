package hub

import (
	"io"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"
)

// PublishHandler allows publisher to broadcast updates to all subscribers
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := authorize(r, h.options.PublisherJWTKey, h.options.PublishAllowedOrigins)
	if err != nil || claims == nil || claims.Mercure.Publish == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	parseFormErr := r.ParseForm()
	if parseFormErr != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	topics := r.PostForm["topic"]
	if len(topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter", http.StatusBadRequest)
		return
	}

	data := r.PostForm.Get("data")
	if data == "" {
		http.Error(w, "Missing \"data\" parameter", http.StatusBadRequest)
		return
	}

	authorizedAlltargets, authorizedTargets := authorizedTargets(claims, true)
	log.Printf("%v", authorizedAlltargets)
	log.Printf("%v", authorizedTargets)

	targets := make(map[string]struct{}, len(r.PostForm["target"]))
	for _, t := range r.PostForm["target"] {
		if !authorizedAlltargets {
			_, ok := authorizedTargets[t]
			if !ok {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		}

		targets[t] = struct{}{}
	}

	var retry uint64
	retryString := r.PostForm.Get("retry")
	if retryString == "" {
		retry = 0
	} else {
		var err error
		retry, err = strconv.ParseUint(retryString, 10, 64)
		if err != nil {
			http.Error(w, "Invalid \"retry\" parameter", http.StatusBadRequest)
			return
		}
	}

	u := &Update{
		Targets: targets,
		Topics:  topics,
		Event:   NewEvent(data, r.PostForm.Get("id"), r.PostForm.Get("type"), retry),
	}

	// Broadcast the update
	h.updates <- newSerializedUpdate(u)
	io.WriteString(w, u.ID)
	log.WithFields(log.Fields{"remote_addr": r.RemoteAddr, "event_id": u.ID}).Info("Update published")
}
