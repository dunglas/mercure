package hub

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
)

// Publisher must be implemented to publish an update
type Publisher interface {
	Publish(hub *Hub, update *Update) error
}

// LocalPublisher dispatch an update locally
type localPublisher struct {
}

// Publish publish an update locally
func (*localPublisher) Publish(h *Hub, u *Update) error {
	if u.ID == "" {
		u.ID = uuid.Must(uuid.NewV4()).String()
	}

	h.DispatchUpdate(u)

	return nil
}

// PublishHandler allows publisher to broadcast updates to all subscribers
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := authorize(r, h.options.PublisherJWTKey, h.options.PublishAllowedOrigins)
	if err != nil || claims == nil || claims.Mercure.Publish == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		log.WithFields(log.Fields{"remote_addr": r.RemoteAddr}).Info(err)
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
		Event:   Event{data, r.PostForm.Get("id"), r.PostForm.Get("type"), retry},
	}

	// Broadcast the update
	err = h.publisher.Publish(h, u)
	if err != nil {
		panic(err)
	}

	io.WriteString(w, u.ID)
	log.WithFields(h.createLogFields(r, u, nil)).Info("Update published")
}
