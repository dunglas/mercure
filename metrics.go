package mercure

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

// hubLabel is the label carrying the hub name on the mercure_hub_* series.
const hubLabel = "hub"

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
	subscribersTotal      prometheus.Counter
	subscribers           prometheus.Gauge
	updatesTotal          prometheus.Counter
	subscriptionsRejected *prometheus.CounterVec

	// hub holds the series labeled with the hub name, nil if the hub is unnamed.
	hub *hubPrometheusMetrics
}

type hubPrometheusMetrics struct {
	subscribersTotal      prometheus.Counter
	subscribers           prometheus.Gauge
	updatesTotal          prometheus.Counter
	subscriptionsRejected *prometheus.CounterVec
}

// NewPrometheusMetrics creates a Prometheus metrics collector.
// Collectors sharing the same registry also share the same series.
func NewPrometheusMetrics(registry prometheus.Registerer) *PrometheusMetrics {
	return NewPrometheusMetricsForHub(registry, "")
}

// NewPrometheusMetricsForHub creates a Prometheus metrics collector for the
// given hub. In addition to the mercure_* series, which are summed across all
// the hubs sharing the registry, it records mercure_hub_* series labeled with
// the hub name. If hub is empty, it behaves like NewPrometheusMetrics.
func NewPrometheusMetricsForHub(registry prometheus.Registerer, hub string) *PrometheusMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	// https://github.com/caddyserver/caddy/pull/6820
	m := &PrometheusMetrics{
		subscriptionsRejected: registerOrReuse(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mercure_subscriptions_rejected_total",
			Help: "Subscriptions rejected by a concurrent stream limit",
		}, []string{"reason"})),
		subscribers: registerOrReuse(registry, prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mercure_subscribers_connected",
			Help: "The current number of running subscribers",
		})),
		subscribersTotal: registerOrReuse(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mercure_subscribers_total",
			Help: "Total number of handled subscribers",
		})),
		updatesTotal: registerOrReuse(registry, prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mercure_updates_total",
			Help: "Total number of handled updates",
		})),
	}

	if hub == "" {
		return m
	}

	m.hub = &hubPrometheusMetrics{
		subscriptionsRejected: registerOrReuse(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mercure_hub_subscriptions_rejected_total",
			Help: "Subscriptions rejected by a concurrent stream limit, per hub",
		}, []string{hubLabel, "reason"})).MustCurryWith(prometheus.Labels{hubLabel: hub}),
		subscribers: registerOrReuse(registry, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mercure_hub_subscribers_connected",
			Help: "The current number of running subscribers, per hub",
		}, []string{hubLabel})).WithLabelValues(hub),
		subscribersTotal: registerOrReuse(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mercure_hub_subscribers_total",
			Help: "Total number of handled subscribers, per hub",
		}, []string{hubLabel})).WithLabelValues(hub),
		updatesTotal: registerOrReuse(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mercure_hub_updates_total",
			Help: "Total number of handled updates, per hub",
		}, []string{hubLabel})).WithLabelValues(hub),
	}

	return m
}

// registerOrReuse registers c, or returns the equivalent collector already
// registered (e.g. by another hub sharing the registry).
func registerOrReuse[T prometheus.Collector](registry prometheus.Registerer, c T) T {
	err := registry.Register(c)
	if err == nil {
		return c
	}

	var are prometheus.AlreadyRegisteredError
	if !errors.As(err, &are) {
		panic(err)
	}

	existing, ok := are.ExistingCollector.(T)
	if !ok {
		panic(err)
	}

	return existing
}

func (m *PrometheusMetrics) SubscriberConnected(_ *LocalSubscriber) {
	m.subscribersTotal.Inc()
	m.subscribers.Inc()

	if m.hub != nil {
		m.hub.subscribersTotal.Inc()
		m.hub.subscribers.Inc()
	}
}

func (m *PrometheusMetrics) SubscriberDisconnected(_ *LocalSubscriber) {
	m.subscribers.Dec()

	if m.hub != nil {
		m.hub.subscribers.Dec()
	}
}

func (m *PrometheusMetrics) UpdatePublished(_ *Update) {
	m.updatesTotal.Inc()

	if m.hub != nil {
		m.hub.updatesTotal.Inc()
	}
}

// SubscriptionRejected records a rejected stream using a bounded reason label
// (total, token, or client), without exposing tokens or client addresses.
// Custom Metrics implementations may optionally implement this method.
func (m *PrometheusMetrics) SubscriptionRejected(reason string) {
	m.subscriptionsRejected.WithLabelValues(reason).Inc()

	if m.hub != nil {
		m.hub.subscriptionsRejected.WithLabelValues(reason).Inc()
	}
}
