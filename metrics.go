package mercure

import (
	"github.com/dunglas/mercure/common"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics interface {
	// SubscriberConnected collects metrics about about subscriber connections.
	SubscriberConnected(s *Subscriber)
	// SubscriberDisconnected collects metrics about subscriber disconnections.
	SubscriberDisconnected(s *Subscriber)
	// UpdatePublished collects metrics about update publications.
	UpdatePublished(u *Update)
}

type NopMetrics struct {
}

func (NopMetrics) SubscriberConnected(s *Subscriber)    {}
func (NopMetrics) SubscriberDisconnected(s *Subscriber) {}
func (NopMetrics) UpdatePublished(s *Update)            {}

// PrometheusMetrics store Hub collected metrics.
type PrometheusMetrics struct {
	registry         prometheus.Registerer
	subscribersTotal *prometheus.CounterVec
	subscribers      *prometheus.GaugeVec
	updatesTotal     *prometheus.CounterVec
}

// NewPrometheusMetrics creates a Prometheus metrics collector.
// This method must be called only one time or it will panic.
func NewPrometheusMetrics(registry prometheus.Registerer) *PrometheusMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	m := &PrometheusMetrics{
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

	m.registry.MustRegister(m.subscribers)
	m.registry.MustRegister(m.subscribersTotal)
	m.registry.MustRegister(m.updatesTotal)

	return m
}

// Register configures the Prometheus registry with all collected metrics.
//
// Deprecated: use the Caddy server module or the standalone library instead.
func (m *PrometheusMetrics) Register(r *mux.Router) {
	// Metrics about current version
	m.registry.MustRegister(common.AppVersion.NewMetricsCollector())

	// Go-specific metrics about the process (GC stats, goroutines, etc.).
	m.registry.MustRegister(prometheus.NewGoCollector())
	// Go-unrelated process metrics (memory usage, file descriptors, etc.).
	m.registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	r.Handle("/metrics", promhttp.HandlerFor(m.registry.(*prometheus.Registry), promhttp.HandlerOpts{})).Methods("GET")
}

func (m *PrometheusMetrics) SubscriberConnected(s *Subscriber) {
	for _, t := range s.Topics {
		m.subscribersTotal.WithLabelValues(t).Inc()
		m.subscribers.WithLabelValues(t).Inc()
	}
}

func (m *PrometheusMetrics) SubscriberDisconnected(s *Subscriber) {
	for _, t := range s.Topics {
		m.subscribers.WithLabelValues(t).Dec()
	}
}

func (m *PrometheusMetrics) UpdatePublished(u *Update) {
	for _, t := range u.Topics {
		m.updatesTotal.WithLabelValues(t).Inc()
	}
}
