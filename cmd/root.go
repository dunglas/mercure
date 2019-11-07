package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/dunglas/mercure/hub"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func init() {
	cobra.OnInitialize(hub.InitConfig)
	fs := rootCmd.Flags()
	hub.SetFlags(fs, viper.GetViper())

	hub.InitLogrus()
}
