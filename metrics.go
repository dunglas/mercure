package mercure

import (
	"errors"
	"fmt"

	"github.com/dunglas/mercure/common"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics store Hub collected metrics.
type Metrics struct {
	registry         prometheus.Registerer
	subscribersTotal *prometheus.CounterVec
	subscribers      *prometheus.GaugeVec
	updatesTotal     *prometheus.CounterVec
}

// newMetrics creates a Prometheus metrics collector.
func newMetrics(registry prometheus.Registerer) (*Metrics, error) {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	m := &Metrics{
		registry: registry,
		subscribersTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mercure_subscribers_total",
				Help: "Total number of handled subscribers",
			},
			[]string{"topic"},
		),
		subscribers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mercure_subscribers",
				Help: "The current number of running subscribers",
			},
			[]string{"topic"},
		),
		updatesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mercure_updates_total",
				Help: "Total number of handled updates",
			},
			[]string{"topic"},
		),
	}

	if err := m.registry.Register(m.subscribers); err != nil && errors.Is(err, prometheus.AlreadyRegisteredError{}) {
		return nil, fmt.Errorf("unable to register collector: %w", err)
	}
	if err := m.registry.Register(m.subscribersTotal); err != nil && errors.Is(err, prometheus.AlreadyRegisteredError{}) {
		return nil, fmt.Errorf("unable to register collector: %w", err)
	}
	if err := m.registry.Register(m.updatesTotal); err != nil && errors.Is(err, prometheus.AlreadyRegisteredError{}) {
		return nil, fmt.Errorf("unable to register collector: %w", err)
	}

	return m, nil
}

// Register configures the Prometheus registry with all collected metrics.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func (m *Metrics) Register(r *mux.Router) {
	// Metrics about current version
	m.registry.MustRegister(common.AppVersion.NewMetricsCollector())

	// Go-specific metrics about the process (GC stats, goroutines, etc.).
	m.registry.MustRegister(prometheus.NewGoCollector())
	// Go-unrelated process metrics (memory usage, file descriptors, etc.).
	m.registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	r.Handle("/metrics", promhttp.HandlerFor(m.registry.(*prometheus.Registry), promhttp.HandlerOpts{})).Methods("GET")
}

// NewSubscriber collects metrics about new subscriber events.
func (m *Metrics) NewSubscriber(s *Subscriber) {
	for _, t := range s.Topics {
		m.subscribersTotal.WithLabelValues(t).Inc()
		m.subscribers.WithLabelValues(t).Inc()
	}
}

// SubscriberDisconnect collects metrics about subscriber disconnection events.
func (m *Metrics) SubscriberDisconnect(s *Subscriber) {
	for _, t := range s.Topics {
		m.subscribers.WithLabelValues(t).Dec()
	}
}

// NewUpdate collects metrics on new update event.
func (m *Metrics) NewUpdate(u *Update) {
	for _, t := range u.Topics {
		m.updatesTotal.WithLabelValues(t).Inc()
	}
}
