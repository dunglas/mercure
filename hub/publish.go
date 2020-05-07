package hub

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"
)

var ErrTargetNotAuthorized = errors.New("target not authorized")

// PublishHandler allows publisher to broadcast updates to all subscribers.
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

	targets, err := getAuthorizedTargets(claims, r.PostForm["target"])
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
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

	u := newUpdate(Event{data, r.PostForm.Get("id"), r.PostForm.Get("type"), retry}, topics, targets)

	// Broadcast the update
	if err := h.transport.Dispatch(u); err != nil {
		panic(err)
	}

	io.WriteString(w, u.ID)
	log.WithFields(addUpdateFields(log.Fields{"remote_addr": r.RemoteAddr}, u, h.config.GetBool("debug"))).Info("Update published")

	h.metrics.NewUpdate(u)
}

func getAuthorizedTargets(claims *claims, t []string) (map[string]struct{}, error) {
	authorizedAlltargets, authorizedTargets := authorizedTargets(claims, true)
	targets := make(map[string]struct{}, len(t))
	for _, t := range t {
		if !authorizedAlltargets {
			_, ok := authorizedTargets[t]
			if !ok {
				return nil, fmt.Errorf("%q: %w", t, ErrTargetNotAuthorized)
			}
		}
		targets[t] = struct{}{}
	}

	return targets, nil
}
