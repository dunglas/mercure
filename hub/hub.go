package hub

import (
	"log"
	"net/http"

	"github.com/spf13/viper"
)

// Hub stores channels with clients currently subscribed and allows to dispatch updates.
type Hub struct {
	config             *viper.Viper
	transport          Transport
	server             *http.Server
	topicSelectorStore *TopicSelectorStore
	metrics            *Metrics
}

// Stop stops disconnect all connected clients.
func (h *Hub) Stop() error {
	return h.transport.Close()
}

// NewHub creates a hub using the Viper configuration.
func NewHub(v *viper.Viper) (*Hub, error) {
	if err := ValidateConfig(v); err != nil {
		return nil, err
	}

	t, err := NewTransport(v)
	if err != nil {
		return nil, err
	}

	return NewHubWithTransport(v, t, NewTopicSelectorStore()), nil
}

// NewHubWithTransport creates a hub.
func NewHubWithTransport(v *viper.Viper, t Transport, tss *TopicSelectorStore) *Hub {
	return &Hub{
		v,
		t,
		nil,
		tss,
		NewMetrics(),
	}
}

// Start is an helper method to start the Mercure Hub.
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
