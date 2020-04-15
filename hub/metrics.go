package hub

import (
	"net/http"
)

// Metrics provides methods to serve metrics
type Metrics interface {
	// Handler returns an http.Handler that is used to server metrics through HTTP
	Handler() http.Handler
}

// NewMetrics create the metrics backend to be used
func NewMetrics() Metrics {
	return &PrometheusMetrics{}
}
