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
		panic("The Reponse Writter must be an instance of Flusher.")
	}

	targets := []string{}
	cookie, err := r.Cookie("mercureAuthorization")
	if err == nil {
		if targets, ok = h.extractTargets(cookie.Value); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
	} else if !h.options.AllowAnonymous {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	topics := r.URL.Query()["topic"]
	if len(topics) == 0 {
		http.Error(w, "Missing \"topic\" parameter.", http.StatusBadRequest)
		return
	}

	var regexps = make([]*regexp.Regexp, len(topics))
	for index, topic := range topics {
		tpl, err := uritemplate.New(topic)
		if nil != err {
			http.Error(w, fmt.Sprintf("\"%s\" is not a valid URI template (RFC6570).", topic), http.StatusBadRequest)
			return
		}
		regexps[index] = tpl.Regexp()
	}

	log.Printf("%s connected.", r.RemoteAddr)
	sendHeaders(w)

	h.sendMissedEvents(w, r, targets, regexps)
	f.Flush()

	// Create a new channel, over which the hub can send can send updates to this subscriber.
	updateChan := make(chan *serializedUpdate)

	// Add this client to the map of those that should
	// receive updates
	h.newSubscribers <- updateChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		h.removedSubscribers <- updateChan
		log.Printf("%s disconnected.", r.RemoteAddr)
	}()

	for {
		serializedUpdate, open := <-updateChan
		if !open {
			break
		}

		// Check authorization
		if !CanDispatch(serializedUpdate.Update, targets, regexps) {
			continue
		}

		fmt.Fprint(w, serializedUpdate.event)
		f.Flush()
	}
}

// extractTargets extracts the subscriber's authorized targets from the JWT
func (h *Hub) extractTargets(encodedToken string) ([]string, bool) {
	token, err := jwt.ParseWithClaims(encodedToken, &claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return h.options.SubscriberJWTKey, nil
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

func (h *Hub) sendMissedEvents(w http.ResponseWriter, r *http.Request, targets []string, topics []*regexp.Regexp) {
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = r.URL.Query().Get("Last-Event-ID")
	}
	if lastEventID == "" {
		return
	}

	if err := h.history.Find(lastEventID, targets, topics, func(u *Update) bool {
		fmt.Fprint(w, u.String())
		return true
	}); err != nil {
		panic(err)
	}
}

// CanDispatch checks if the update can be dispatched according to the given criterias
func CanDispatch(update *Update, subscriberTargets []string, subsriberTopics []*regexp.Regexp) bool {
	return isAuthorized(subscriberTargets, update.Targets) && isSubscribedToUpdate(subsriberTopics, update.Topics)
}

// isAuthorized checks if the subscriber can access to at least one of the update's intended targets
func isAuthorized(subscriberTargets []string, updateTargets map[string]struct{}) bool {
	if len(updateTargets) == 0 {
		return true
	}

	for _, t := range subscriberTargets {
		if _, ok := updateTargets[t]; ok {
			return true
		}
	}

	return false
}

// isSubscribedToUpdate checks if the subscriber has subscribed to this update
func isSubscribedToUpdate(subscriberTopics []*regexp.Regexp, updateTopics []string) bool {
	// Add a global cache here
	for _, st := range subscriberTopics {
		for _, ut := range updateTopics {
			if st.MatchString(ut) {
				return true
			}
		}
	}

	return false
}
