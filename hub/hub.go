package hub

import "log"

// Partially based on https://github.com/kljensen/golang-html5-sse-example

// Hub stores channels with clients currently subcribed
type Hub struct {
	publisherJWTKey    []byte
	subscriberJWTKey   []byte
	subscribers        map[chan Resource]struct{}
	newSubscribers     chan chan Resource
	removedSubscribers chan chan Resource
	resources          chan Resource
}

// NewHub creates a hub
func NewHub(publisherJWTKey []byte, subscriberJWTKey []byte) *Hub {
	return &Hub{
		publisherJWTKey,
		subscriberJWTKey,
		make(map[chan Resource]struct{}),
		make(chan (chan Resource)),
		make(chan (chan Resource)),
		make(chan Resource),
	}
}

// Start starts the hub
func (h *Hub) Start() {
	go func() {
		for {
			select {

			case s := <-h.newSubscribers:
				h.subscribers[s] = struct{}{}

			case s := <-h.removedSubscribers:
				delete(h.subscribers, s)
				close(s)

			case content, ok := <-h.resources:
				for s := range h.subscribers {
					if ok {
						s <- content
					} else {
						close(s)
					}
				}
				if ok {
					log.Printf("Broadcast resource \"%s\".", content.IRI)
				} else {
					return
				}
			}
		}
	}()
}

// Stop stops disconnect all connected clients
func (h *Hub) Stop() {
	close(h.resources)
}
