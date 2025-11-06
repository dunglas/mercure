package mercure

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

// Publish broadcasts the given update to all subscribers.
// The id field of the Update instance can be updated by the underlying Transport.
func (h *Hub) Publish(ctx context.Context, update *Update) error {
	if err := h.transport.Dispatch(ctx, update); err != nil {
		return err //nolint:wrapcheck
	}

	h.metrics.UpdatePublished(update)
	h.logger.DebugContext(ctx, "Update published", slog.Any("update", update))

	return nil
}

// PublishHandler allows publisher to broadcast updates to all subscribers.
//
//nolint:funlen
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	var (
		claims *claims
		err    error
	)

	if h.publisherJWTKeyFunc != nil {
		claims, err = h.authorize(r, true)
		if err != nil || claims == nil || claims.Mercure.Publish == nil {
			h.httpAuthorizationError(w, r, err)

			return
		}
	}

	if r.ParseForm() != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}

	topics := r.PostForm["topic"]
	if len(topics) == 0 {
		http.Error(w, `Missing "topic" parameter`, http.StatusBadRequest)

		return
	}

	var retry uint64
	if retryString := r.PostForm.Get("retry"); retryString != "" {
		if retry, err = strconv.ParseUint(retryString, 10, 64); err != nil {
			http.Error(w, `Invalid "retry" parameter`, http.StatusBadRequest)

			return
		}
	}

	private := len(r.PostForm["private"]) != 0
	if claims != nil && !canDispatch(h.topicSelectorStore, topics, claims.Mercure.Publish) {
		if private {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			return
		}

		if h.isBackwardCompatiblyEnabledWith(7) {
			h.logger.Info(`Deprecated: posting public updates to topics not listed in the "mercure.publish" JWT claim is deprecated since the version 7 of the protocol, use '["*"]' as value to allow publishing on all topics.`)
		} else {
			h.logger.Info(`Unsupported: posting public updates to topics not listed in the "mercure.publish" JWT claim is not supported anymore, use '["*"]' as value to allow publishing on all topics or enable backward compatibility with the version 7 of the protocol.`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			return
		}
	}

	u := &Update{
		Topics:  topics,
		Private: private,
		Debug:   h.debug,
		Event:   Event{r.PostForm.Get("data"), r.PostForm.Get("id"), r.PostForm.Get("type"), retry},
	}

	ctx := r.Context()
	// Broadcast the update
	if err := h.transport.Dispatch(ctx, u); err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		h.logger.ErrorContext(ctx, "Update published", slog.Any("update", u), slog.Any("error", err))

		return
	}

	if _, err := io.WriteString(w, u.ID); err != nil {
		h.logger.WarnContext(ctx, "Failed to write publish response", slog.Any("update", u), slog.Any("error", err))

		return
	}

	h.metrics.UpdatePublished(u)
	h.logger.DebugContext(ctx, "Update published", slog.Any("update", u), slog.Any("error", err))
}
