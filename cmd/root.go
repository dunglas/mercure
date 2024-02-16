package cmd

import (
	"fmt"
	"log"

	"github.com/dunglas/mercure"
	"github.com/dunglas/mercure/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{ //nolint:gochecknoglobals
	Use:   "mercure",
	Short: "Start the Mercure Hub",
	Long: `Mercure is a protocol allowing to push data updates to web browsers and
other HTTP clients in a convenient, fast, reliable and battery-efficient way.
The Mercure Hub is the reference implementation of the Mercure protocol.

Go to https://mercure.rocks for more information!`,
	Run: func(_ *cobra.Command, _ []string) {
		mercure.Start() //nolint:staticcheck
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func init() { //nolint:gochecknoinits
	log.Println("/!\\ This Mercure binary is deprecated, use the binary based on Caddy Server instead! See https://mercure.rocks/docs/UPGRADE")

	v := viper.GetViper()
	cobra.OnInitialize(func() {
		mercure.InitConfig(v) //nolint:staticcheck
	})
	fs := rootCmd.Flags()
	mercure.SetFlags(fs, v) //nolint:staticcheck

	appVersion := common.AppVersion
	rootCmd.Version = appVersion.Shortline()

	versionTemplate := fmt.Sprintf("Mercure.rocks Hub version %s\n%s\n", rootCmd.Version, appVersion.ChangelogURL())
	rootCmd.SetVersionTemplate(versionTemplate)
}
