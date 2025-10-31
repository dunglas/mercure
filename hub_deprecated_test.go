//go:build deprecated_server

package mercure

import (
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHubValidationError(t *testing.T) {
	assert.Panics(t, func() {
		_, _ = NewHubFromViper(viper.New())
	})
}

func TestNewHubTransportValidationError(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("publisher_jwt_key", "foo")
	v.Set("jwt_key", "bar")
	v.Set("transport_url", "foo://")

	assert.Panics(t, func() {
		_, _ = NewHubFromViper(viper.New())
	})
}

func TestStartCrash(t *testing.T) {
	t.Parallel()

	if os.Getenv("BE_START_CRASH") == "1" {
		Start()

		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestStartCrash") //nolint:gosec

	cmd.Env = append(os.Environ(), "BE_START_CRASH=1")
	err := cmd.Run()

	var e *exec.ExitError
	require.ErrorAs(t, err, &e)
	assert.False(t, e.Success())
}

func TestNewHubDeprecated(t *testing.T) {
	t.Parallel()

	h := createDummy(t)

	assert.IsType(t, &viper.Viper{}, h.config)
}

func setDeprecatedOptions(tb testing.TB, h *Hub) {
	tb.Helper()

	h.config = viper.New()
	h.config.Set("addr", testAddr)
	h.config.Set("metrics_addr", testMetricsAddr)
}
