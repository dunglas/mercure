package config

import (
	"github.com/spf13/viper"
	assert2 "github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestGet_ENV(t *testing.T) {
	_ = os.Setenv("XDDR","400")
	assert := assert2.New(t)

	assert.NotEqual("",GetString("XDDR"))
}

func TestGetString_NotFound(t *testing.T) {
	assert := assert2.New(t)

	assert.Equal("",GetString("VDDR"))
}

func TestGet_File(t *testing.T) {
	assert := assert2.New(t)

	assert.NotNil(Get("TESTX"))
}

func Test_BadConfigFile(t *testing.T) {
	assert := assert2.New(t)
	defer func() {
		assert.NotNil(recover())
	}()
	viper.SetConfigName("bad_config")
	setupConfig()
}

func Test_LoadFromEnviroment(t *testing.T){
	viper.SetConfigName("non_exist_config")
	setupConfig()
}
