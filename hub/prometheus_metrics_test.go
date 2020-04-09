package hub

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPrometheusMetrics(t *testing.T) {
	p := NewPrometheusMetrics()
	assert.Implements(t, (*Metrics)(nil), p)
}

func TestPrometheusMetricsHandler(t *testing.T) {
	p := NewPrometheusMetrics()
	assert.Implements(t, (*http.Handler)(nil), p.Handler())
}
