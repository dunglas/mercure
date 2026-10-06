package mercure

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// debuggerConfig is the runtime configuration the bundled UI fetches on load to
// adapt itself: whether the insecure playground is enabled (so it shows the
// playground panels and prefills a token), whether anonymous subscription is
// allowed (so it can subscribe with no token), whether the subscription API is
// enabled (so it can offer the active-subscriptions panel), and the name of the
// cookie the playground auth flow sets.
type debuggerConfig struct {
	Playground    bool   `json:"playground"`
	Anonymous     bool   `json:"anonymous"`
	Subscriptions bool   `json:"subscriptions"`
	CookieName    string `json:"cookieName"`
}

// DebuggerConfigHandler serves the bundled UI's runtime configuration as JSON.
// It exposes only hub state the UI needs to adapt itself, never a token.
// Registered whenever the debugger is enabled (see WithDebugger).
func (h *Hub) DebuggerConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	config := debuggerConfig{
		Playground:    h.playground,
		Anonymous:     h.anonymous,
		Subscriptions: h.subscriptions,
		CookieName:    h.cookieName,
	}

	if err := json.NewEncoder(w).Encode(config); err != nil && h.logger.Enabled(r.Context(), slog.LevelInfo) {
		h.logger.LogAttrs(r.Context(), slog.LevelInfo, "Failed to write debugger config response", slog.Any("error", err))
	}
}

// PlaygroundTokenHandler returns a freshly minted access token for the debugger
// UI to prefill, with the aud claim set to the resource identifier derived for
// this request (so it is valid on whatever public URL the playground answers on).
//
// INSECURE: the token grants publish and subscribe on every topic; it exists
// only to make the playground self-serve. Registered only when the playground is
// enabled and a signing callback is configured (see WithPlayground, WithPlaygroundTokenFunc).
//
// EXPERIMENTAL. Not covered by the backward compatibility promise.
func (h *Hub) PlaygroundTokenHandler(w http.ResponseWriter, r *http.Request) {
	resourceIdentifier, _ := h.requestIdentity(r)

	token, err := h.playgroundTokenFunc(resourceIdentifier)
	if err != nil {
		if h.logger.Enabled(r.Context(), slog.LevelError) {
			h.logger.LogAttrs(r.Context(), slog.LevelError, "Failed to mint the playground token", slog.Any("error", err))
		}

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	_, _ = io.WriteString(w, token)
}
