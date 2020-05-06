package common

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
)

type AppVersionInfo struct {
	Version   string
	BuildDate string
}

var AppVersion AppVersionInfo //nolint:gochecknoglobals

// these variables are dynamically set at build.
var version = "dev"
var buildDate = "" //nolint:gochecknoglobals

func (v *AppVersionInfo) Shortline() string {
	if v.BuildDate == "" {
		return v.Version
	}

	return fmt.Sprintf("%s (%s)", v.Version, v.BuildDate)
}

func (v *AppVersionInfo) ChangelogURL() string {
	path := "https://github.com/dunglas/mercure"
	r := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[\w.]+)?$`)
	if !r.MatchString(v.Version) {
		return fmt.Sprintf("%s/releases/latest", path)
	}

	return fmt.Sprintf("%s/releases/tag/v%s", path, strings.TrimPrefix(v.Version, "v"))
}

func init() { //nolint:gochecknoinits
	if version == "dev" {
		info, ok := debug.ReadBuildInfo()
		if ok && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}

	version = strings.TrimPrefix(version, "v")

	AppVersion = AppVersionInfo{
		Version:   version,
		BuildDate: buildDate,
	}
}
