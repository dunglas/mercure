package hub

import (
	"net/http"
	"sync"

	"github.com/yosida95/uritemplate"
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
	options      *Options
	transport    Transport
	server       *http.Server
	uriTemplates uriTemplates
}

// Stop stops disconnect all connected clients
func (h *Hub) Stop() error {
	return h.transport.Close()
}

// NewHubFromEnv creates a hub using the configuration set in env vars
func NewHubFromEnv() (*Hub, error) {
	options, err := NewOptionsFromEnv()
	if err != nil {
		return nil, err
	}

	transport, err := NewTransport(options)
	if err != nil {
		return nil, err
	}

	return NewHub(transport, options), nil
}

// NewHub creates a hub
func NewHub(transport Transport, options *Options) *Hub {
	return &Hub{
		options,
		transport,
		nil,
		uriTemplates{m: make(map[string]*templateCache)},
	}
}
