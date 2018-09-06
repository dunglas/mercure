package hub

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	jwt "github.com/dgrijalva/jwt-go"
)

// PublishHandler allows publisher to broadcast updates to all subscribers
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

	topics := r.Form["topic"]
	if len(topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter", http.StatusBadRequest)
		return
	}

	data := r.Form.Get("data")
	if data == "" {
		http.Error(w, "Missing \"data\" parameter", http.StatusBadRequest)
		return
	}

	targets := make(map[string]struct{}, len(r.Form["target"]))
	for _, t := range r.Form["target"] {
		targets[t] = struct{}{}
	}

	var retry uint64
	retryString := r.Form.Get("retry")
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

	// Broadcast the update
	h.updates <- newSerializedUpdate(NewUpdate(topics, targets, data, r.Form.Get("id"), r.Form.Get("type"), retry))
}

// Checks the validity of the JWT
func (h *Hub) isAuthorizationValid(authorizationHeader string) bool {
	if len(authorizationHeader) < 48 || authorizationHeader[:7] != "Bearer " {
		return false
	}

	token, _ := jwt.Parse(authorizationHeader[7:], func(token *jwt.Token) (interface{}, error) {
		log.Println(token.Header["alg"])
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return h.options.PublisherJWTKey, nil
	})

	return token.Valid
}
