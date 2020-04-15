package hub

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics implements the Metrics interface
type PrometheusMetrics struct {
}

// NewPrometheusMetrics create a new PrometheusMetrics
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{}
}

// Handler provide the Prometheus HTTP handler to serve metrics
func (m *PrometheusMetrics) Handler() http.Handler {
	return promhttp.Handler()
}
