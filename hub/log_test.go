package hub

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitLogrus(t *testing.T) {
	viper.Set("debug", true)

	viper.Set("log_format", "JSON")
	InitLogrus()
	assert.Equal(t, logrus.DebugLevel, logrus.GetLevel())

	viper.Set("log_format", "FLUENTD")
	InitLogrus()
}
