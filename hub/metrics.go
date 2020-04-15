package hub

import (
	"net/http"
)

type Metrics interface {
	Handler() http.Handler
}

func NewMetrics() Metrics {
	return &PrometheusMetrics{}
}
