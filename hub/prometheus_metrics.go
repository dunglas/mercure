package hub

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusMetrics struct {
}

func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{}
}

func (m *PrometheusMetrics) Handler() http.Handler {
	return promhttp.Handler()
}
