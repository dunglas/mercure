package common

import (
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type AppVersionInfo struct {
	Version      string
	BuildDate    string
	Commit       string
	GoVersion    string
	OS           string
	Architecture string
}

var AppVersion AppVersionInfo //nolint:gochecknoglobals

// these variables are dynamically set at build.
var (
	version   = "dev"
	buildDate = "" //nolint:gochecknoglobals
	commit    = "" //nolint:gochecknoglobals
)

func (v *AppVersionInfo) Shortline() string {
	shortline := v.Version

	if v.Commit != "" {
		shortline += ", commit " + v.Commit
	}

	if v.BuildDate != "" {
		shortline += ", built at " + v.BuildDate
	}

	return shortline
}

func (v *AppVersionInfo) ChangelogURL() string {
	const path = "https://github.com/dunglas/mercure"

	if v.Version == "dev" {
		return path + "/releases/latest"
	}

	return path + "/releases/tag/v" + strings.TrimPrefix(v.Version, "v")
}

func (v *AppVersionInfo) NewMetricsCollector() *prometheus.GaugeVec {
	labels := map[string]string{
		"version":      v.Version,
		"built_at":     v.BuildDate,
		"commit":       v.Commit,
		"go_version":   v.GoVersion,
		"os":           v.OS,
		"architecture": v.Architecture,
	}

	labelNames := make([]string, 0, len(labels))
	for n := range labels {
		labelNames = append(labelNames, n)
	}

	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mercure_version_info",
			Help: "A metric with a constant '1' value labeled by different build stats fields.",
		},
		labelNames,
	)
	buildInfo.With(labels).Set(1)

	return buildInfo
}

func init() { //nolint:gochecknoinits
	if version == "dev" {
		info, ok := debug.ReadBuildInfo()
		if ok && info.Main.Version != "(devel)" && info.Main.Version != "" {
			version = info.Main.Version
		}
	}

	version = strings.TrimPrefix(version, "v")

	AppVersion = AppVersionInfo{
		Version:      version,
		BuildDate:    buildDate,
		Commit:       commit,
		GoVersion:    runtime.Version(),
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}
