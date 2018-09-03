package hub

import "log"

// Hub stores channels with clients currently subcribed
type Hub struct {
	publisherJWTKey    []byte
	subscriberJWTKey   []byte
	allowAnonymous     bool
	subscribers        map[chan Update]struct{}
	newSubscribers     chan chan Update
	removedSubscribers chan chan Update
	updates            chan Update
}

// NewHub creates a hub
func NewHub(publisherJWTKey, subscriberJWTKey []byte, allowAnonymous bool) *Hub {
	return &Hub{
		publisherJWTKey,
		subscriberJWTKey,
		allowAnonymous,
		make(map[chan Update]struct{}),
		make(chan (chan Update)),
		make(chan (chan Update)),
		make(chan Update),
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

			case update, ok := <-h.updates:
				for s := range h.subscribers {
					if ok {
						s <- update
					} else {
						close(s)
					}
				}
				if ok {
					log.Printf("Broadcast topics \"%s\".", update.Topics)
				} else {
					return
				}
			}
		}
	}()
}

// Stop stops disconnect all connected clients
func (h *Hub) Stop() {
	close(h.updates)
}
