package mercure

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNumberOfRunningSubscribers(t *testing.T) {
	t.Parallel()

	m := NewPrometheusMetrics(nil)

	tms := &TopicMatcherStore{}
	logger := slog.Default()

	s1 := NewLocalSubscriber("", logger, tms)
	s1.setMatchers(stringsToExactMatchers([]string{"topic1", "topic2"}), stringsToExactMatchers(nil))
	m.SubscriberConnected(s1)
	assertGaugeValue(t, 1.0, m.subscribers)

	s2 := NewLocalSubscriber("", logger, tms)
	s2.setMatchers(stringsToExactMatchers([]string{"topic2"}), stringsToExactMatchers(nil))
	m.SubscriberConnected(s2)
	assertGaugeValue(t, 2.0, m.subscribers)

	m.SubscriberDisconnected(s1)
	assertGaugeValue(t, 1.0, m.subscribers)

	m.SubscriberDisconnected(s2)
	assertGaugeValue(t, 0.0, m.subscribers)
}

func TestTotalNumberOfHandledSubscribers(t *testing.T) {
	t.Parallel()

	m := NewPrometheusMetrics(nil)

	tms := &TopicMatcherStore{}
	logger := slog.Default()

	s1 := NewLocalSubscriber("", logger, tms)
	s1.setMatchers(stringsToExactMatchers([]string{"topic1", "topic2"}), stringsToExactMatchers(nil))
	m.SubscriberConnected(s1)
	assertCounterValue(t, 1.0, m.subscribersTotal)

	s2 := NewLocalSubscriber("", logger, tms)
	s2.setMatchers(stringsToExactMatchers([]string{"topic2"}), stringsToExactMatchers(nil))
	m.SubscriberConnected(s2)
	assertCounterValue(t, 2.0, m.subscribersTotal)

	m.SubscriberDisconnected(s1)
	m.SubscriberDisconnected(s2)

	assertCounterValue(t, 2.0, m.subscribersTotal)
}

func TestTotalOfHandledUpdates(t *testing.T) {
	t.Parallel()

	m := NewPrometheusMetrics(nil)

	m.UpdatePublished(&Update{
		Topics: []string{"topic1"},
	})
	m.UpdatePublished(&Update{
		Topics: []string{"topic2"},
	})
	m.UpdatePublished(&Update{
		Topics: []string{"topic2"},
	})
	m.UpdatePublished(&Update{
		Topics: []string{"topic3"},
	})

	assertCounterValue(t, 4.0, m.updatesTotal)
}

func TestMetricsOfSeveralHubsSharingARegistry(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewPedanticRegistry()
	a := NewPrometheusMetricsForHub(registry, "hub-a")
	b := NewPrometheusMetricsForHub(registry, "hub-b")

	tms := &TopicMatcherStore{}
	logger := slog.Default()

	a.SubscriberConnected(NewLocalSubscriber("", logger, tms))

	for range 2 {
		b.SubscriberConnected(NewLocalSubscriber("", logger, tms))
	}

	s := NewLocalSubscriber("", logger, tms)
	b.SubscriberConnected(s)
	b.SubscriberDisconnected(s)

	b.UpdatePublished(&Update{Topics: []string{"topic1"}})
	a.SubscriptionRejected("total")

	// Provisioning the same hub again (e.g. the same name in another site block)
	// must not panic and must feed the same series.
	NewPrometheusMetricsForHub(registry, "hub-a").UpdatePublished(&Update{Topics: []string{"topic1"}})

	assert.Equal(t, map[string]float64{
		// Summed across all the hubs sharing the registry.
		`mercure_subscribers_connected{}`:                      3,
		`mercure_subscribers_total{}`:                          4,
		`mercure_subscriptions_rejected_total{reason="total"}`: 1,
		`mercure_updates_total{}`:                              2,

		`mercure_hub_subscribers_connected{hub="hub-a"}`:                       1,
		`mercure_hub_subscribers_connected{hub="hub-b"}`:                       2,
		`mercure_hub_subscribers_total{hub="hub-a"}`:                           1,
		`mercure_hub_subscribers_total{hub="hub-b"}`:                           3,
		`mercure_hub_subscriptions_rejected_total{hub="hub-a",reason="total"}`: 1,
		`mercure_hub_updates_total{hub="hub-a"}`:                               1,
		`mercure_hub_updates_total{hub="hub-b"}`:                               1,
	}, gatherSeries(t, registry))
}

// TestMetricsWithoutHubName checks that unnamed hubs expose no hub label and
// no mercure_hub_* series.
func TestMetricsWithoutHubName(t *testing.T) {
	t.Parallel()

	for _, newMetrics := range map[string]func(prometheus.Registerer) *PrometheusMetrics{
		"NewPrometheusMetrics":             NewPrometheusMetrics,
		"NewPrometheusMetricsForHub(\"\")": func(r prometheus.Registerer) *PrometheusMetrics { return NewPrometheusMetricsForHub(r, "") },
	} {
		registry := prometheus.NewPedanticRegistry()
		m := newMetrics(registry)

		tms := &TopicMatcherStore{}
		logger := slog.Default()

		s := NewLocalSubscriber("", logger, tms)
		m.SubscriberConnected(s)
		m.SubscriberConnected(NewLocalSubscriber("", logger, tms))
		m.SubscriberDisconnected(s)
		m.UpdatePublished(&Update{Topics: []string{"topic1"}})
		m.SubscriptionRejected("token")

		assert.Equal(t, map[string]float64{
			`mercure_subscribers_connected{}`:                      1,
			`mercure_subscribers_total{}`:                          2,
			`mercure_subscriptions_rejected_total{reason="token"}`: 1,
			`mercure_updates_total{}`:                              1,
		}, gatherSeries(t, registry))
	}
}

// gatherSeries returns the value of every series in the registry, keyed by
// name and labels.
func gatherSeries(t *testing.T, registry prometheus.Gatherer) map[string]float64 {
	t.Helper()

	families, err := registry.Gather()
	require.NoError(t, err)

	series := make(map[string]float64)

	for _, family := range families {
		for _, metric := range family.GetMetric() {
			labels := make([]string, 0, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels = append(labels, fmt.Sprintf("%s=%q", label.GetName(), label.GetValue()))
			}

			key := family.GetName() + "{" + strings.Join(labels, ",") + "}"

			switch {
			case metric.GetGauge() != nil:
				series[key] = metric.GetGauge().GetValue()
			case metric.GetCounter() != nil:
				series[key] = metric.GetCounter().GetValue()
			}
		}
	}

	return series
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
