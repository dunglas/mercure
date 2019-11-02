package hub

import (
	"net/http"
	"sync"

	"github.com/yosida95/uritemplate"
	bolt "go.etcd.io/bbolt"
)

// uriTemplates caches uritemplate.Template to improve memory and CPU usage
type uriTemplates struct {
	sync.RWMutex
	m map[string]*templateCache
}

type templateCache struct {
	// counter stores the number of subsribers currently using this topic
	counter uint32
	// the uritemplate.Template instance, of nil if it's a raw string
	template *uritemplate.Template
}

// Hub stores channels with clients currently subscribed and allows to dispatch updates
type Hub struct {
	subscribers        map[chan *serializedUpdate]struct{}
	updates            chan *serializedUpdate
	options            *Options
	newSubscribers     chan chan *serializedUpdate
	removedSubscribers chan chan *serializedUpdate
	publisher          Publisher
	history            History
	server             *http.Server
	uriTemplates       uriTemplates
	lastSentEventID    string
	sync.RWMutex
}

// Start starts the hub
func (h *Hub) Start() {
	go func() {
		for {
			select {

			case s := <-h.newSubscribers:
				h.Lock()
				h.subscribers[s] = struct{}{}
				h.Unlock()

			case s := <-h.removedSubscribers:
				h.Lock()
				delete(h.subscribers, s)
				h.Unlock()
				close(s)

			case serializedUpdate, ok := <-h.updates:
				h.Lock()
				if ok {
					if err := h.history.Add(serializedUpdate.Update); err != nil {
						panic(err)
					}

					h.lastSentEventID = serializedUpdate.ID
				}

				for s := range h.subscribers {
					if ok {
						s <- serializedUpdate
					} else {
						close(s)
					}
				}
				h.Unlock()

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

// DispatchUpdate dispatches an update to all subscribers
func (h *Hub) DispatchUpdate(u *Update) {
	h.updates <- newSerializedUpdate(u)
}

// NewHubFromEnv creates a hub using the configuration set in env vars
func NewHubFromEnv() (*Hub, *bolt.DB, error) {
	options, err := NewOptionsFromEnv()
	if err != nil {
		return nil, nil, err
	}

	db, err := bolt.Open(options.DBPath, 0600, nil)
	if err != nil {
		return nil, nil, err
	}

	return NewHub(&localPublisher{}, &boltHistory{db, options}, options), db, nil
}

// NewHub creates a hub
func NewHub(publisher Publisher, history History, options *Options) *Hub {
	return &Hub{
		subscribers:        make(map[chan *serializedUpdate]struct{}),
		updates:            make(chan *serializedUpdate),
		options:            options,
		newSubscribers:     make(chan (chan *serializedUpdate)),
		removedSubscribers: make(chan (chan *serializedUpdate)),
		publisher:          publisher,
		history:            history,
		uriTemplates:       uriTemplates{m: make(map[string]*templateCache)},
	}
}
