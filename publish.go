package mercure

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

type updateContextKeyType struct{}

var UpdateContextKey updateContextKeyType //nolint:gochecknoglobals

// Publish broadcasts the given update to all subscribers.
// The id field of the Update instance can be updated by the underlying Transport.
func (h *Hub) Publish(ctx context.Context, update *Update) error {
	ctx = context.WithValue(ctx, UpdateContextKey, update)

	if err := h.transport.Dispatch(ctx, update); err != nil && h.logger.Enabled(ctx, slog.LevelError) {
		h.logger.LogAttrs(ctx, slog.LevelError, "Failed to dispatch update", slog.Any("error", err))

		return err //nolint:wrapcheck
	}

	h.metrics.UpdatePublished(update)

	if h.logger.Enabled(ctx, slog.LevelDebug) {
		h.logger.LogAttrs(ctx, slog.LevelDebug, "Update published")
	}

	return nil
}

// PublishHandler allows publisher to broadcast updates to all subscribers.
//
//nolint:funlen
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	var claims *claims

	if h.publisherJWTKeyFunc != nil {
		var err error

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
		var err error
		if retry, err = strconv.ParseUint(retryString, 10, 64); err != nil {
			http.Error(w, `Invalid "retry" parameter`, http.StatusBadRequest)

			return
		}
	}

	ctx := r.Context()

	private := len(r.PostForm["private"]) != 0
	if claims != nil && !canDispatch(h.topicSelectorStore, topics, claims.Mercure.Publish) { //nolint:nestif
		if private {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			return
		}

		infoEnabled := h.logger.Enabled(ctx, slog.LevelInfo)
		if h.isBackwardCompatiblyEnabledWith(7) {
			if infoEnabled {
				h.logger.LogAttrs(ctx, slog.LevelInfo, `Deprecated: posting public updates to topics not listed in the "mercure.publish" JWT claim is deprecated since the version 7 of the protocol, use '["*"]' as value to allow publishing on all topics.`)
			}
		} else {
			if infoEnabled {
				h.logger.LogAttrs(ctx, slog.LevelInfo, `Unsupported: posting public updates to topics not listed in the "mercure.publish" JWT claim is not supported anymore, use '["*"]' as value to allow publishing on all topics or enable backward compatibility with the version 7 of the protocol.`)
			}

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
	ctx = context.WithValue(context.WithoutCancel(ctx), UpdateContextKey, u)

	// Broadcast the update
	if err := h.transport.Dispatch(ctx, u); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		if h.logger.Enabled(ctx, slog.LevelError) {
			h.logger.LogAttrs(ctx, slog.LevelError, "Failed to dispatch update", slog.Any("error", err))
		}

		return
	}

	if _, err := io.WriteString(w, u.ID); err != nil {
		if h.logger.Enabled(ctx, slog.LevelInfo) {
			h.logger.LogAttrs(ctx, slog.LevelInfo, "Failed to write publish response", slog.Any("error", err))
		}

		return
	}

	h.metrics.UpdatePublished(u)

	if h.logger.Enabled(ctx, slog.LevelInfo) {
		h.logger.LogAttrs(ctx, slog.LevelInfo, "Update published")
	}
}
