package hub

import (
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/yosida95/uritemplate"
)

// SubscribeHandler create a keep alive connection and send the events to the subscribers
func (h *Hub) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Panic("The Reponse Writter must be an instance of Flusher.")
		return
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

	// Create a new channel, over which the hub can send can send resources to this subscriber.
	resourceChan := make(chan Resource)

	// Add this client to the map of those that should
	// receive updates
	h.newSubscribers <- resourceChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		// Remove this client from the map of attached clients
		// when `EventHandler` exits.
		h.removedSubscribers <- resourceChan
		log.Printf("%s disconnected.", r.RemoteAddr)
	}()

	for {
		// Read from our resourceChan.
		resource, open := <-resourceChan

		if !open {
			// If our resourceChan was closed, this means that the client has disconnected.
			break
		}

		match := false
		for _, r := range regexps {
			if r.MatchString(resource.IRI) {
				match = true
				break
			}
		}
		if !match {
			continue
		}

		fmt.Fprint(w, "event: mercure\n")
		fmt.Fprintf(w, "id: %s\n", resource.IRI)
		fmt.Fprint(w, resource.Data)

		f.Flush()
	}
}
