package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionInfo(t *testing.T) {
	v := AppVersionInfo{
		Version:   "dev",
		BuildDate: "",
	}

	assert.Equal(t, v.Shortline(), "dev")
	assert.Equal(t, v.ChangelogURL(), "https://github.com/dunglas/mercure/releases/latest")
}

func TestVersionInfoWithBuildDate(t *testing.T) {
	v := AppVersionInfo{
		Version:   "1.0.0",
		BuildDate: "2020-05-03T18:42:44Z",
		Commit:    "",
	}

	assert.Equal(t, v.Shortline(), "1.0.0, built at 2020-05-03T18:42:44Z")
	assert.Equal(t, v.ChangelogURL(), "https://github.com/dunglas/mercure/releases/tag/v1.0.0")
}

func TestVersionInfoWithCommit(t *testing.T) {
	v := AppVersionInfo{
		Version:   "1.0.0",
		BuildDate: "",
		Commit:    "96ee2b9",
	}

	assert.Equal(t, v.Shortline(), "1.0.0, commit 96ee2b9")
	assert.Equal(t, v.ChangelogURL(), "https://github.com/dunglas/mercure/releases/tag/v1.0.0")
}

func TestVersionInfoWithBuildDateAndCommit(t *testing.T) {
	v := AppVersionInfo{
		Version:   "1.0.0",
		BuildDate: "2020-05-03T18:42:44Z",
		Commit:    "96ee2b9",
	}

	assert.Equal(t, v.Shortline(), "1.0.0, commit 96ee2b9, built at 2020-05-03T18:42:44Z")
	assert.Equal(t, v.ChangelogURL(), "https://github.com/dunglas/mercure/releases/tag/v1.0.0")
}
