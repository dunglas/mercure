package common

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestVersionInfo(t *testing.T) {
	v := AppVersionInfo{
		Version:      "dev",
		BuildDate:    "",
		Commit:       "",
		GoVersion:    "go1.14.2",
		OS:           "linux",
		Architecture: "amd64",
	}

	assert.Equal(t, "dev", v.Shortline())
	assert.Equal(t, "https://github.com/dunglas/mercure/releases/latest", v.ChangelogURL())
}

func TestVersionInfoWithBuildDate(t *testing.T) {
	v := AppVersionInfo{
		Version:      "1.0.0",
		BuildDate:    "2020-05-03T18:42:44Z",
		Commit:       "",
		GoVersion:    "go1.14.2",
		OS:           "linux",
		Architecture: "amd64",
	}

	assert.Equal(t, "1.0.0, built at 2020-05-03T18:42:44Z", v.Shortline())
	assert.Equal(t, "https://github.com/dunglas/mercure/releases/tag/v1.0.0", v.ChangelogURL())
}

func TestVersionInfoWithCommit(t *testing.T) {
	v := AppVersionInfo{
		Version:      "1.0.0",
		BuildDate:    "",
		Commit:       "96ee2b9",
		GoVersion:    "go1.14.2",
		OS:           "linux",
		Architecture: "amd64",
	}

	assert.Equal(t, "1.0.0, commit 96ee2b9", v.Shortline())
	assert.Equal(t, "https://github.com/dunglas/mercure/releases/tag/v1.0.0", v.ChangelogURL())
}

func TestVersionInfoWithBuildDateAndCommit(t *testing.T) {
	v := AppVersionInfo{
		Version:      "1.0.0",
		BuildDate:    "2020-05-03T18:42:44Z",
		Commit:       "96ee2b9",
		GoVersion:    "go1.14.2",
		OS:           "linux",
		Architecture: "amd64",
	}

	assert.Equal(t, "1.0.0, commit 96ee2b9, built at 2020-05-03T18:42:44Z", v.Shortline())
	assert.Equal(t, "https://github.com/dunglas/mercure/releases/tag/v1.0.0", v.ChangelogURL())
}

func TestVersionMetricsCollectorInitialization(t *testing.T) {
	var metricOut dto.Metric

	v := AppVersionInfo{
		Version:      "1.0.0",
		BuildDate:    "2020-05-03T18:42:44Z",
		Commit:       "96ee2b9",
		GoVersion:    "go1.14.2",
		OS:           "linux",
		Architecture: "amd64",
	}

	c := v.NewMetricsCollector()

	labelValues := map[string]string{
		"version":      v.Version,
		"built_at":     v.BuildDate,
		"commit":       v.Commit,
		"go_version":   v.GoVersion,
		"os":           v.OS,
		"architecture": v.Architecture,
	}
	m, err := c.GetMetricWith(labelValues)
	if err != nil {
		t.Fatal(err)
	}

	err = m.Write(&metricOut)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1.0, metricOut.GetGauge().GetValue()) //nolint:testifylint
}
