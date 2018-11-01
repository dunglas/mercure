package hub

import (
	"net/http"
	"sync"
)

type serializedUpdate struct {
	*Update
	event string
}

func newSerializedUpdate(u *Update) *serializedUpdate {
	return &serializedUpdate{u, u.String()}
}

type subscribers struct {
	sync.RWMutex
	m map[chan *serializedUpdate]struct{}
}

// Hub stores channels with clients currently subcribed
type Hub struct {
	options            *Options
	subscribers        subscribers
	newSubscribers     chan chan *serializedUpdate
	removedSubscribers chan chan *serializedUpdate
	updates            chan *serializedUpdate
	history            History
	server             *http.Server
}

// NewHubFromEnv creates a hub fusing the configuration set in env vars
func NewHubFromEnv(history History) (*Hub, error) {
	options, err := NewOptionsFromEnv()
	if err != nil {
		return nil, err
	}

	return NewHub(history, options), nil
}

// NewHub creates a hub
func NewHub(history History, options *Options) *Hub {
	return &Hub{
		options,
		subscribers{m: make(map[chan *serializedUpdate]struct{})},
		make(chan (chan *serializedUpdate)),
		make(chan (chan *serializedUpdate)),
		make(chan *serializedUpdate),
		history,
		nil,
	}
}

// Start starts the hub
func (h *Hub) Start() {
	go func() {
		for {
			select {

			case s := <-h.newSubscribers:
				h.subscribers.Lock()
				h.subscribers.m[s] = struct{}{}
				h.subscribers.Unlock()

			case s := <-h.removedSubscribers:
				h.subscribers.Lock()
				delete(h.subscribers.m, s)
				h.subscribers.Unlock()
				close(s)

			case serializedUpdate, ok := <-h.updates:
				if ok {
					if err := h.history.Add(serializedUpdate.Update); err != nil {
						panic(err)
					}
				}

				h.subscribers.RLock()
				for s := range h.subscribers.m {
					if ok {
						s <- serializedUpdate
					} else {
						close(s)
					}
				}
				h.subscribers.RUnlock()

				if !ok {
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
