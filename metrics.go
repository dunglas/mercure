package mercure

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

const metricsPath = "/metrics"

type Metrics interface {
	// SubscriberConnected collects metrics about subscriber connections.
	SubscriberConnected(s *LocalSubscriber)
	// SubscriberDisconnected collects metrics about subscriber disconnections.
	SubscriberDisconnected(s *LocalSubscriber)
	// UpdatePublished collects metrics about update publications.
	UpdatePublished(u *Update)
}

type NopMetrics struct{}

func (NopMetrics) SubscriberConnected(_ *LocalSubscriber)    {}
func (NopMetrics) SubscriberDisconnected(_ *LocalSubscriber) {}
func (NopMetrics) UpdatePublished(_ *Update)                 {}

// PrometheusMetrics store Hub collected metrics.
type PrometheusMetrics struct {
	registry         prometheus.Registerer
	subscribersTotal prometheus.Counter
	subscribers      prometheus.Gauge
	updatesTotal     prometheus.Counter
}

// NewPrometheusMetrics creates a Prometheus metrics collector.
// This method must be called only one time, or it will panic.
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

	// https://github.com/caddyserver/caddy/pull/6820
	if err := m.registry.Register(m.subscribers); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	if err := m.registry.Register(m.subscribersTotal); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	if err := m.registry.Register(m.updatesTotal); err != nil &&
		!errors.As(err, &prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}

	return m
}

func (m *PrometheusMetrics) SubscriberConnected(_ *LocalSubscriber) {
	m.subscribersTotal.Inc()
	m.subscribers.Inc()
}

func (m *PrometheusMetrics) SubscriberDisconnected(_ *LocalSubscriber) {
	m.subscribers.Dec()
}

func (m *PrometheusMetrics) UpdatePublished(_ *Update) {
	m.updatesTotal.Inc()
}
