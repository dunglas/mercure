package hub

import "log"

// Partially based on https://github.com/kljensen/golang-html5-sse-example

// Hub stores channels with clients currently subcribed
type Hub struct {
	subscribers        map[chan Resource]bool
	newSubscribers     chan chan Resource
	removedSubscribers chan chan Resource
	resources          chan Resource
	publisherJwtKey    []byte
	subscriberJwtKey   []byte
}

// NewHub creates a hub
func NewHub(publisherJwtKey []byte, subscriberJwtKey []byte) Hub {
	return Hub{
		make(map[chan Resource]bool),
		make(chan (chan Resource)),
		make(chan (chan Resource)),
		make(chan Resource),
		publisherJwtKey,
		subscriberJwtKey,
	}
}

// Start starts the hub
func (h *Hub) Start() {
	go func() {
		for {
			select {

			case s := <-h.newSubscribers:
				h.subscribers[s] = true

			case s := <-h.removedSubscribers:
				delete(h.subscribers, s)
				close(s)

			case content := <-h.resources:
				for s := range h.subscribers {
					s <- content
				}
				log.Printf("Broadcast resource \"%s\".", content.IRI)
			}
		}
	}()
}
