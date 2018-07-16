package hub

import (
	"fmt"
	"net/http"

	jwt "github.com/dgrijalva/jwt-go"
)

// PublishHandler allows publisher to broadcast resources to all subscribers
func (h *Hub) PublishHandler(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthorizationValid(r.Header.Get("Authorization")) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	parseFormErr := r.ParseForm()
	if parseFormErr != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	iri := r.Form.Get("iri")
	if iri == "" {
		http.Error(w, "Missing \"iri\" parameter", http.StatusBadRequest)
		return
	}

	data := r.Form.Get("data")
	if data == "" {
		http.Error(w, "Missing \"data\" parameter", http.StatusBadRequest)
		return
	}

	targets := make(map[string]bool, len(r.Form["target[]"]))
	for _, t := range r.Form["target[]"] {
		targets[t] = true
	}

	// Broadcast the resource
	h.resources <- NewResource(iri, data, targets)
}

// Checks the validity of the JWT
func (h *Hub) isAuthorizationValid(authorizationHeader string) bool {
	if len(authorizationHeader) < 75 || authorizationHeader[:7] != "Bearer " {
		return false
	}

	token, _ := jwt.Parse(authorizationHeader[7:], func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return h.publisherJwtKey, nil
	})

	return token.Valid
}
