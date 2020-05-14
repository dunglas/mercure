package hub

import (
	"io"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"
)

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

	var retry uint64
	retryString := r.PostForm.Get("retry")
	if retryString != "" {
		retry, err = strconv.ParseUint(retryString, 10, 64)
		if err != nil {
			http.Error(w, "Invalid \"retry\" parameter", http.StatusBadRequest)
			return
		}
	}

	private := len(r.PostForm["private"]) != 0
	if private && !canDispatch(h.topicSelectorStore, topics, claims.Mercure.Publish) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	u := newUpdate(
		topics,
		private,
		Event{
			r.PostForm.Get("data"),
			r.PostForm.Get("id"),
			r.PostForm.Get("type"),
			retry,
		},
	)

	// Broadcast the update
	if err := h.transport.Dispatch(u); err != nil {
		panic(err)
	}

	io.WriteString(w, u.ID)
	log.WithFields(addUpdateFields(log.Fields{"remote_addr": r.RemoteAddr}, u, h.config.GetBool("debug"))).Info("Update published")

	h.metrics.NewUpdate(u)
}
