package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDevelopmentVersionInfo(t *testing.T) {
	v := AppVersionInfo{
		Version:   "dev",
		BuildDate: "",
	}

	assert.Equal(t, v.Shortline(), "dev")
	assert.Equal(t, v.ChangelogURL(), "https://github.com/dunglas/mercure/releases/latest")
}

func TestTaggedVersionInfo(t *testing.T) {
	v := AppVersionInfo{
		Version:   "1.0.0",
		BuildDate: "2020-05-03T18:42:44Z",
	}

	assert.Equal(t, v.Shortline(), "1.0.0 (2020-05-03T18:42:44Z)")
	assert.Equal(t, v.ChangelogURL(), "https://github.com/dunglas/mercure/releases/tag/v1.0.0")
}
