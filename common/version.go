package common

import (
	"fmt"
	"runtime/debug"
	"strings"
)

type AppVersionInfo struct {
	Version   string
	BuildDate string
	Commit    string
}

var AppVersion AppVersionInfo //nolint:gochecknoglobals

// these variables are dynamically set at build.
var version = "dev"
var buildDate = "" //nolint:gochecknoglobals
var commit = ""    //nolint:gochecknoglobals

func (v *AppVersionInfo) Shortline() string {
	shortline := v.Version

	if v.Commit != "" {
		shortline += fmt.Sprintf(", commit %s", v.Commit)
	}

	if v.BuildDate != "" {
		shortline += fmt.Sprintf(", built at %s", v.BuildDate)
	}

	return shortline
}

func (v *AppVersionInfo) ChangelogURL() string {
	path := "https://github.com/dunglas/mercure"

	if v.Version == "dev" {
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
		Commit:    commit,
	}
}
