package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/yosida95/uritemplate"
)

type content struct {
	iri  string
	data string
}

type Broker struct {
	// key: client, value: useless
	clients map[chan content]bool

	// Channel into which new clients can be pushed
	newClients chan chan content

	// Channel into which disconnected clients should be pushed
	defunctClients chan chan content

	// Channel into which messages are pushed to be broadcast out
	// to attahed clients.
	contents chan content
}

func (b *Broker) Start() {
	go func() {
		for {
			select {

			case s := <-b.newClients:
				b.clients[s] = true
				log.Println("Added new client")

			case s := <-b.defunctClients:
				delete(b.clients, s)
				close(s)
				log.Println("Removed client")

			case content := <-b.contents:
				for s := range b.clients {
					s <- content
				}
				log.Printf("Broadcast message to %d clients", len(b.clients))
			}
		}
	}()
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Server-sent events https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#Sending_events_from_the_server
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")

	// Disable cache, even for old browsers and proxies
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expire", "0")

	// NGINX support https://www.nginx.com/resources/wiki/start/topics/examples/x-accel/#x-accel-buffering
	w.Header().Set("X-Accel-Buffering", "no")

	// Create a new channel, over which the broker can
	// send this client messages.
	contentChan := make(chan content)

	// Add this client to the map of those that should
	// receive updates
	b.newClients <- contentChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		// Remove this client from the map of attached clients
		// when `EventHandler` exits.
		b.defunctClients <- contentChan
		log.Println("HTTP connection just closed.")
	}()

	for {
		// Read from our messageChan.
		content, open := <-contentChan

		if !open {
			// If our messageChan was closed, this means that the client has
			// disconnected.
			break
		}

		match := false
		for _, r := range regexps {
			log.Printf("%v", r)
			if r.MatchString(content.iri) {
				match = true
				break
			}
		}
		if !match {
			continue
		}

		fmt.Fprint(w, "event: mercure\n")
		fmt.Fprintf(w, "id: %s\n", content.iri)
		fmt.Fprint(w, content.data)

		f.Flush()
	}

	log.Println("Finished HTTP request at ", r.URL.Path)
}

func (b *Broker) PublishHandler(w http.ResponseWriter, r *http.Request) {
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

	// Encode the message: replace newlines by "\ndata: ", https://www.w3.org/TR/eventsource/#dispatchMessage
	encodedBody := fmt.Sprintf("data: %s\n\n", strings.Replace(data, "\n", "\ndata: ", -1))
	b.contents <- content{iri, encodedBody}

	fmt.Fprintf(w, "Published a new message: %s", iri)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("Missing template file.")
	}

	t.Execute(w, nil)
}

// Main routine
//
func main() {
	b := &Broker{
		make(map[chan content]bool),
		make(chan (chan content)),
		make(chan (chan content)),
		make(chan content),
	}
	b.Start()

	http.Handle("/events/", b)

	http.Handle("/publish", http.HandlerFunc(b.PublishHandler))
	http.Handle("/", http.HandlerFunc(IndexHandler))

	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = ":80"
	}
	log.Printf("Mercure started on %s.\n", listen)

	http.ListenAndServe(listen, nil)
}
