package hub

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	subscribersTotal *prometheus.CounterVec
	subscribers      *prometheus.GaugeVec
}

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
	}
}

func (m *Metrics) Register(r *mux.Router) {
	registry := prometheus.NewRegistry()

	// Metrics about the Hub
	registry.MustRegister(m.subscribers)
	registry.MustRegister(m.subscribersTotal)
	// Go-specific metrics about the process (GC stats, goroutines, etc.).
	registry.MustRegister(prometheus.NewGoCollector())
	// Go-unrelated process metrics (memory usage, file descriptors, etc.).
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	r.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{})).Methods("GET")
}

func (m *Metrics) NewSubscriber(topics []string) {
	for _, t := range topics {
		m.subscribersTotal.WithLabelValues(t).Inc()
		m.subscribers.WithLabelValues(t).Inc()
	}
}

func (m *Metrics) SubscriberDisconnect(topics []string) {
	for _, t := range topics {
		m.subscribers.WithLabelValues(t).Dec()
	}
}

