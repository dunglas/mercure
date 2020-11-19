package hub

import (
	"io"
	"net/http"
	"strconv"

	"go.uber.org/zap"
)

// PublishHandler allows publisher to broadcast updates to all subscribers.
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	var claims *claims
	if h.publisherJWTConfig != nil {
		var err error
		claims, err = authorize(r, h.publisherJWTConfig, h.publishOrigins)
		if err != nil || claims == nil || claims.Mercure.Publish == nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			h.logger.Info("Topic selectors not matched or not provided", zap.String("remote_addr", r.RemoteAddr), zap.Error(err))

			return
		}
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
	if retryString := r.PostForm.Get("retry"); retryString != "" {
		var err error
		if retry, err = strconv.ParseUint(retryString, 10, 64); err != nil {
			http.Error(w, "Invalid \"retry\" parameter", http.StatusBadRequest)

			return
		}
	}

	private := len(r.PostForm["private"]) != 0
	if private && !canDispatch(h.topicSelectorStore, topics, claims.Mercure.Publish) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

		return
	}

	u := &Update{
		Topics:  topics,
		Private: private,
		Debug:   h.debug,
		Event:   Event{r.PostForm.Get("data"), r.PostForm.Get("id"), r.PostForm.Get("type"), retry},
	}

	// Broadcast the update
	if err := h.transport.Dispatch(u); err != nil {
		panic(err)
	}

	io.WriteString(w, u.ID)
	h.logger.Info("Update published", zap.Object("update", u), zap.String("remote_addr", r.RemoteAddr))

	if h.metrics != nil {
		h.metrics.NewUpdate(u)
	}
}
