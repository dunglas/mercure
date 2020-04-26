package hub

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestNumberOfRunningSubscribers(t *testing.T) {
	m := NewMetrics()

	m.NewSubscriber([]string{"topic1", "topic2"})
	assertGaugeLabelValue(t, 1.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 1.0, m.subscribers, "topic2")

	m.NewSubscriber([]string{"topic2"})
	assertGaugeLabelValue(t, 1.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 2.0, m.subscribers, "topic2")

	m.SubscriberDisconnect([]string{"topic1"})
	assertGaugeLabelValue(t, 0.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 2.0, m.subscribers, "topic2")

	m.SubscriberDisconnect([]string{"topic2"})
	m.SubscriberDisconnect([]string{"topic2"})
	assertGaugeLabelValue(t, 0.0, m.subscribers, "topic1")
	assertGaugeLabelValue(t, 0.0, m.subscribers, "topic2")
}

func TestTotalNumberOfHandledSubscribers(t *testing.T) {
	m := NewMetrics()

	m.NewSubscriber([]string{"topic1", "topic2"})
	assertCounterValue(t, 1.0, m.subscribersTotal, "topic1")
	assertCounterValue(t, 1.0, m.subscribersTotal, "topic2")

	m.NewSubscriber([]string{"topic2"})
	assertCounterValue(t, 1.0, m.subscribersTotal, "topic1")
	assertCounterValue(t, 2.0, m.subscribersTotal, "topic2")

	m.SubscriberDisconnect([]string{"topic2"})
	assertCounterValue(t, 1.0, m.subscribersTotal, "topic1")
	assertCounterValue(t, 2.0, m.subscribersTotal, "topic2")
}

func TestTotalOfHandledUpdates(t *testing.T) {
	m := NewMetrics()

	m.NewUpdate([]string{"topic1", "topic2"})
	m.NewUpdate([]string{"topic2", "topic3"})
	m.NewUpdate([]string{"topic2"})
	m.NewUpdate([]string{"topic3"})

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
