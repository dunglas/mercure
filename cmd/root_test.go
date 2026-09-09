//go:build deprecated_server && deprecated_transport

package cmd

import (
	"testing"

	"github.com/dunglas/mercure/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCmdVersion(t *testing.T) {
	assert.NotEmpty(t, rootCmd.Version)
	assert.Equal(t, common.AppVersion.Shortline(), rootCmd.Version)
}

func TestExecutePanicsOnError(t *testing.T) {
	orig := rootCmd
	t.Cleanup(func() { rootCmd = orig })

	rootCmd = &cobra.Command{
		RunE: func(_ *cobra.Command, _ []string) error {
			return assert.AnError
		},
	}

	assert.Panics(t, Execute)
}
