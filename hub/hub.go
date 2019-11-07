package hub

import (
	"log"
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

// NewHub creates a hub using the Viper configuration
func NewHub(v *viper.Viper) (*Hub, error) {
	if err := ValidateConfig(v); err != nil {
		return nil, err
	}

	t, err := NewTransport(v)
	if err != nil {
		return nil, err
	}

	return NewHubWithTransport(v, t), nil
}

// NewHubWithTransport creates a hub
func NewHubWithTransport(v *viper.Viper, t Transport) *Hub {
	return &Hub{
		v,
		t,
		nil,
		uriTemplates{m: make(map[string]*templateCache)},
	}
}

// Start is an helper method to start the Mercure Hub
func Start() {
	h, err := NewHub(viper.GetViper())
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if err = h.Stop(); err != nil {
			log.Fatalln(err)
		}
	}()

	h.Serve()
}
