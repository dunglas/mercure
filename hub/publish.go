package hub

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
)

func (h *Hub) dispatch(u *Update) error {
	if u.ID == "" {
		u.ID = uuid.Must(uuid.NewV4()).String()
	}

	return h.transport.Write(u)
}

// PublishHandler allows publisher to broadcast updates to all subscribers
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := authorize(r, h.getJWTKey(publisherRole), h.getJWTAlgorithm(publisherRole), h.config.GetStringSlice("publish_allowed_origins"))
	if err != nil || claims == nil || claims.Mercure.Publish == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		log.WithFields(log.Fields{"remote_addr": r.RemoteAddr}).Info(err)
		return
	}

	if r.ParseForm() != nil {
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
	if retryString != "" {
		retry, err = strconv.ParseUint(retryString, 10, 64)
		if err != nil {
			http.Error(w, "Invalid \"retry\" parameter", http.StatusBadRequest)
			return
		}
	}

	u := &Update{
		Targets: targets,
		Topics:  topics,
		Event:   Event{data, r.PostForm.Get("id"), r.PostForm.Get("type"), retry},
	}

	// Broadcast the update
	if err := h.dispatch(u); err != nil {
		panic(err)
	}

	io.WriteString(w, u.ID)
	log.WithFields(h.createLogFields(r, u, nil)).Info("Update published")
}
