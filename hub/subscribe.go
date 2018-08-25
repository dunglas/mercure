package hub

import (
	"fmt"
	"log"
	"net/http"
	"regexp"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/yosida95/uritemplate"
)

type claims struct {
	MercureTargets []string `json:"mercureTargets"`
	jwt.StandardClaims
}

// SubscribeHandler create a keep alive connection and send the events to the subscribers
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Panic("The Reponse Writter must be an instance of Flusher.")
		return
	}

	targets := []string{}
	cookie, err := r.Cookie("mercureAuthorization")
	if err == nil {
		if targets, ok = h.extractTargets(cookie.Value); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
	}

	iris := r.URL.Query()["iri[]"]
	if len(iris) == 0 {
		http.Error(w, "Missing \"iri[]\" parameters.", http.StatusBadRequest)
		return
	}

	var regexps = make([]*regexp.Regexp, len(iris))
	for index, iri := range iris {
		tpl, err := uritemplate.New(iri)
		if nil != err {
			http.Error(w, fmt.Sprintf("\"%s\" is not a valid URI template (RFC6570).", iri), http.StatusBadRequest)
			return
		}
		regexps[index] = tpl.Regexp()
	}

	log.Printf("%s connected.", r.RemoteAddr)
	sendHeaders(w)

	// Create a new channel, over which the hub can send can send resources to this subscriber.
	resourceChan := make(chan Resource)

	// Add this client to the map of those that should
	// receive updates
	h.newSubscribers <- resourceChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		h.removedSubscribers <- resourceChan
		log.Printf("%s disconnected.", r.RemoteAddr)
	}()

	for {
		resource, open := <-resourceChan
		if !open {
			break
		}

		// Check authorization
		if !isAuthorized(targets, resource.Targets) || !isSubscribedToResource(regexps, resource.IRI) {
			continue
		}

		fmt.Fprint(w, "event: mercure\n")
		fmt.Fprintf(w, "id: %s\n", resource.RevID)
		fmt.Fprint(w, resource.Data)

		f.Flush()
	}
}

// extractTargets extracts the subscriber's authorized targets from the JWT
func (h *Hub) extractTargets(encodedToken string) ([]string, bool) {
	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return h.subscriberJWTKey, nil
	})

	if err != nil {
		return nil, false
	}

	if claims, ok := token.Claims.(*claims); ok && token.Valid {
		return claims.MercureTargets, true
	}

	return nil, false
}

// sendHeaders send correct HTTP headers to create a keep-alive connection
func sendHeaders(w http.ResponseWriter) {
	// Keep alive, useful only for HTTP 1 clients https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Keep-Alive
	w.Header().Set("Connection", "keep-alive")

	// Server-sent events https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Sending_events_from_the_server
	w.Header().Set("Content-Type", "text/event-stream")

	// Disable cache, even for old browsers and proxies
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expire", "0")

	// NGINX support https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/#x-accel-buffering
	w.Header().Set("X-Accel-Buffering", "no")
}

// isAuthorized checks if the subscriber can access to at least one of the resource's intended targets
func isAuthorized(subscriberTargets []string, resourceTargets map[string]struct{}) bool {
	if len(resourceTargets) == 0 {
		return true
	}

	for _, t := range subscriberTargets {
		if _, ok := resourceTargets[t]; ok {
			return true
		}
	}

	return false
}

// isSubscribedToResource checks if the subscriber has subscribed to this resource
func isSubscribedToResource(regexps []*regexp.Regexp, resourceIri string) bool {
	for _, r := range regexps {
		if r.MatchString(resourceIri) {
			return true
		}
	}

	return false
}
