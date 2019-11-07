package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/dunglas/mercure/hub"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{ //nolint:gochecknoglobals
	Use:   "mercure",
	Short: "Start the Mercure Hub",
	Long: `Mercure is a protocol allowing to push data updates to web browsers and
other HTTP clients in a convenient, fast, reliable and battery-efficient way.
The Mercure Hub is the reference implementation of the Mercure protocol.

Go to https://mercure.rocks for more information!`,
	Run: func(cmd *cobra.Command, args []string) {
		hub.Start()
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func init() { //nolint:gochecknoinits
	v := viper.GetViper()
	cobra.OnInitialize(func() {
		hub.InitConfig(v)
	})
	fs := rootCmd.Flags()
	hub.SetFlags(fs, v)

	hub.InitLogrus()
}
