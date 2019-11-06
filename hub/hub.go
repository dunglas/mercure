package hub

import (
	"net/http"
	"sync"

	"github.com/spf13/viper"
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
	config       *viper.Viper
	transport    Transport
	server       *http.Server
	uriTemplates uriTemplates
}

// Stop stops disconnect all connected clients
func (h *Hub) Stop() error {
	return h.transport.Close()
}

// NewHubFromConfig creates a hub using the Viper configuration
func NewHubFromConfig() (*Hub, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, err
	}

	transport, err := NewTransport(config)
	if err != nil {
		return nil, err
	}

	return NewHub(transport, config), nil
}

// NewHub creates a hub
func NewHub(transport Transport, config *viper.Viper) *Hub {
	return &Hub{
		config,
		transport,
		nil,
		uriTemplates{m: make(map[string]*templateCache)},
	}
}
