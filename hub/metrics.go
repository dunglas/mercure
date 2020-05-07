package hub

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics store Hub collected metrics.
type Metrics struct {
	subscribersTotal *prometheus.CounterVec
	subscribers      *prometheus.GaugeVec
	updatesTotal     *prometheus.CounterVec
}

// NewMetrics creates a Prometheus metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		subscribersTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mercure_subcribers_total",
				Help: "Total number of handled subsribers",
			},
			[]string{"topic"},
		),
		subscribers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mercure_subcribers",
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
}

// Register configures the Prometheus registry with all collected metrics.
func (m *Metrics) Register(r *mux.Router) {
	registry := prometheus.NewRegistry()

	// Metrics about the Hub
	registry.MustRegister(m.subscribers)
	registry.MustRegister(m.subscribersTotal)
	registry.MustRegister(m.updatesTotal)

	// Go-specific metrics about the process (GC stats, goroutines, etc.).
	registry.MustRegister(prometheus.NewGoCollector())
	// Go-unrelated process metrics (memory usage, file descriptors, etc.).
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	r.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{})).Methods("GET")
}

// NewSubscriber collects metrics about new subscriber events.
func (m *Metrics) NewSubscriber(s *Subscriber) {
	for _, t := range s.topics {
		m.subscribersTotal.WithLabelValues(t).Inc()
		m.subscribers.WithLabelValues(t).Inc()
	}
}

// SubscriberDisconnect collects metrics about subscriber disconnection events.
func (m *Metrics) SubscriberDisconnect(s *Subscriber) {
	for _, t := range s.topics {
		m.subscribers.WithLabelValues(t).Dec()
	}
}

// NewUpdate collects metrics on new update event.
func (m *Metrics) NewUpdate(u *Update) {
	for _, t := range u.Topics {
		m.updatesTotal.WithLabelValues(t).Inc()
	}
}
