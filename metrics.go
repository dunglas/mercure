package mercure

import (
	"net/http"

	"github.com/dunglas/mercure/common"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsPath = "/metrics"

type Metrics interface {
	// SubscriberConnected collects metrics about subscriber connections.
	SubscriberConnected(s *Subscriber)
	// SubscriberDisconnected collects metrics about subscriber disconnections.
	SubscriberDisconnected(s *Subscriber)
	// UpdatePublished collects metrics about update publications.
	UpdatePublished(u *Update)
}

type NopMetrics struct{}

func (NopMetrics) SubscriberConnected(_ *Subscriber)    {}
func (NopMetrics) SubscriberDisconnected(_ *Subscriber) {}
func (NopMetrics) UpdatePublished(_ *Update)            {}

// PrometheusMetrics store Hub collected metrics.
type PrometheusMetrics struct {
	registry         prometheus.Registerer
	subscribersTotal prometheus.Counter
	subscribers      prometheus.Gauge
	updatesTotal     prometheus.Counter
}

// NewPrometheusMetrics creates a Prometheus metrics collector.
// This method must be called only one time or it will panic.
func NewPrometheusMetrics(registry prometheus.Registerer) *PrometheusMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	m := &PrometheusMetrics{
		registry: registry,
		subscribersTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "mercure_subscribers_total",
				Help: "Total number of handled subscribers",
			},
		),
		subscribers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "mercure_subscribers_connected",
				Help: "The current number of running subscribers",
			},
		),
		updatesTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "mercure_updates_total",
				Help: "Total number of handled updates",
			},
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

	r.Handle(metricsPath, promhttp.HandlerFor(m.registry.(*prometheus.Registry), promhttp.HandlerOpts{})).Methods(http.MethodGet)
}

func (m *PrometheusMetrics) SubscriberConnected(_ *Subscriber) {
	m.subscribersTotal.Inc()
	m.subscribers.Inc()
}

func (m *PrometheusMetrics) SubscriberDisconnected(_ *Subscriber) {
	m.subscribers.Dec()
}

func (m *PrometheusMetrics) UpdatePublished(_ *Update) {
	m.updatesTotal.Inc()
}
