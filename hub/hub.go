package hub

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Hub stores channels with clients currently subscribed and allows to dispatch updates.
type Hub struct {
	config             *viper.Viper
	logger             Logger
	transport          Transport
	server             *http.Server
	metricsServer      *http.Server
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

	var (
		logger Logger
		err    error
	)

	if v.GetBool("debug") {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		return nil, fmt.Errorf("can't initialize zap logger: %w", err)
	}

	t, err := NewTransport(v, logger)
	if err != nil {
		return nil, err
	}

	return NewHubWithTransport(v, t, logger, NewTopicSelectorStore()), nil
}

// NewHubWithTransport creates a hub.
func NewHubWithTransport(v *viper.Viper, t Transport, logger Logger, tss *TopicSelectorStore) *Hub {
	return &Hub{
		v,
		logger,
		t,
		nil,
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
