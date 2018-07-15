package hub

import (
	"fmt"
	"net/http"
)

// PublishHandler allows publisher to broadcast resources to all subscribers
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid request")

		return
	}

	iri := r.Form.Get("iri")
	if iri == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing \"iri\" parameter")

		return
	}

	data := r.Form.Get("data")
	if data == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing \"data\" parameter")

		return
	}

	// Broadcast the resource
	h.resources <- NewResource(iri, data)
}
