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

	sst := NewTopicSelectorStore()

	s1 := NewSubscriber("", zap.NewNop(), sst)
	s1.Topics = []string{"topic1", "topic2"}
	m.SubscriberConnected(s1)
	assertGaugeLabelValue(t, 1.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 1.0, m.subscribers, "topic2")

	s2 := NewSubscriber("", zap.NewNop(), sst)
	s2.Topics = []string{"topic2"}
	m.SubscriberConnected(s2)
	assertGaugeLabelValue(t, 1.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 2.0, m.subscribers, "topic2")

	m.SubscriberDisconnected(s1)
	assertGaugeLabelValue(t, 0.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 1.0, m.subscribers, "topic2")

	m.SubscriberDisconnected(s2)
	assertGaugeLabelValue(t, 0.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 0.0, m.subscribers, "topic2")
}

func TestTotalNumberOfHandledSubscribers(t *testing.T) {
	m := NewPrometheusMetrics(nil)

	sst := NewTopicSelectorStore()

	s1 := NewSubscriber("", zap.NewNop(), sst)
	s1.Topics = []string{"topic1", "topic2"}
	m.SubscriberConnected(s1)
	assertCounterValue(t, 1.0, m.subscribersTotal, "topic1")
	assertCounterValue(t, 1.0, m.subscribersTotal, "topic2")

	s2 := NewSubscriber("", zap.NewNop(), sst)
	s2.Topics = []string{"topic2"}
	m.SubscriberConnected(s2)
	assertCounterValue(t, 1.0, m.subscribersTotal, "topic1")
	assertCounterValue(t, 2.0, m.subscribersTotal, "topic2")

	m.SubscriberDisconnected(s1)
	m.SubscriberDisconnected(s2)

	assertCounterValue(t, 1.0, m.subscribersTotal, "topic1")
	assertCounterValue(t, 2.0, m.subscribersTotal, "topic2")
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

	assertCounterValue(t, 1.0, m.updatesTotal, "topic1")
	assertCounterValue(t, 3.0, m.updatesTotal, "topic2")
	assertCounterValue(t, 2.0, m.updatesTotal, "topic3")
}

func assertGaugeLabelValue(t *testing.T, v float64, g *prometheus.GaugeVec, l string) {
	var metricOut dto.Metric

	m, err := g.GetMetricWithLabelValues(l)
	if err != nil {
		t.Fatal(err)
	}

	err = m.Write(&metricOut)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, v, *metricOut.Gauge.Value)
}

func assertCounterValue(t *testing.T, v float64, c *prometheus.CounterVec, l string) {
	var metricOut dto.Metric

	m, err := c.GetMetricWithLabelValues(l)
	if err != nil {
		t.Fatal(err)
	}

	err = m.Write(&metricOut)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, v, *metricOut.Counter.Value)
}
