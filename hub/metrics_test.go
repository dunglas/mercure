package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics()
	assert.IsType(t, &PrometheusMetrics{}, metrics)
}
