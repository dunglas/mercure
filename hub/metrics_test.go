package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	metrics, err := NewMetrics()
	assert.Nil(t, err)
	assert.IsType(t, &PrometheusMetrics{}, metrics)
}
