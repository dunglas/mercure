package mercure

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNumberOfRunningSubscribers(t *testing.T) {
	m := NewPrometheusMetrics(nil)

	s1 := NewSubscriber("", zap.NewNop())
	s1.SetTopics([]string{"topic1", "topic2"}, nil)
	m.SubscriberConnected(s1)
	assertGaugeValue(t, 1.0, m.subscribers)

	s2 := NewSubscriber("", zap.NewNop())
	s2.SetTopics([]string{"topic2"}, nil)
	m.SubscriberConnected(s2)
	assertGaugeValue(t, 2.0, m.subscribers)

	m.SubscriberDisconnected(s1)
	assertGaugeValue(t, 1.0, m.subscribers)

	m.SubscriberDisconnected(s2)
	assertGaugeValue(t, 0.0, m.subscribers)
}

func TestTotalNumberOfHandledSubscribers(t *testing.T) {
	m := NewPrometheusMetrics(nil)

	s1 := NewSubscriber("", zap.NewNop())
	s1.SetTopics([]string{"topic1", "topic2"}, nil)
	m.SubscriberConnected(s1)
	assertCounterValue(t, 1.0, m.subscribersTotal)

	s2 := NewSubscriber("", zap.NewNop())
	s2.SetTopics([]string{"topic2"}, nil)
	m.SubscriberConnected(s2)
	assertCounterValue(t, 2.0, m.subscribersTotal)

	m.SubscriberDisconnected(s1)
	m.SubscriberDisconnected(s2)

	assertCounterValue(t, 2.0, m.subscribersTotal)
}

func TestTotalOfHandledUpdates(t *testing.T) {
	m := NewPrometheusMetrics(nil)

	m.UpdatePublished(&Update{
		Topics: []string{"topic1", "topic2"},
	})
	m.UpdatePublished(&Update{
		Topics: []string{"topic2", "topic3"},
	})
	m.UpdatePublished(&Update{
		Topics: []string{"topic2"},
	})
	m.UpdatePublished(&Update{
		Topics: []string{"topic3"},
	})

	assertCounterValue(t, 4.0, m.updatesTotal)
}

func assertGaugeValue(t *testing.T, v float64, g prometheus.Gauge) {
	t.Helper()

	var metricOut dto.Metric
	if err := g.Write(&metricOut); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, v, metricOut.GetGauge().GetValue()) //nolint:testifylint
}

func assertCounterValue(t *testing.T, v float64, c prometheus.Counter) {
	t.Helper()

	var metricOut dto.Metric
	if err := c.Write(&metricOut); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, v, metricOut.GetCounter().GetValue()) // nolint:testifylint
}
