package config

import (
	assert2 "github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestGet_ENV(t *testing.T) {
	_ = os.Setenv("XDDR","400")
	assert := assert2.New(t)

	assert.NotEqual("",GetString("XDDR"))
}

func TestGet_File(t *testing.T) {
	assert := assert2.New(t)

	assert.NotNil(Get("TESTX"))
}
