package cmd

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/dunglas/mercure/hub"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is the running Hub version and is dynamically set at build
var version = "dev" //nolint:gochecknoglobals

// buildDate stores the build date and is dynamically set at build
var buildDate = "" //nolint:gochecknoglobals

// rootCmd represents the base command when called without any subcommands.
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

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}

func init() { //nolint:gochecknoinits
	v := viper.GetViper()
	cobra.OnInitialize(func() {
		hub.InitConfig(v)
		hub.InitLogrus()
	})
	fs := rootCmd.Flags()
	hub.SetFlags(fs, v)

	rootCmd.Version = buildVersion()

	versionTemplate := fmt.Sprintf("mercure version %s\n%s\n", rootCmd.Version, changelogURL(version))
	rootCmd.SetVersionTemplate(versionTemplate)
}

func buildVersion() string {
	if version == "dev" {
		info, ok := debug.ReadBuildInfo()
		if ok && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}

	version = strings.TrimPrefix(version, "v")

	if buildDate == "" {
		return version
	}

	return fmt.Sprintf("%s (%s)", version, buildDate)
}

func changelogURL(version string) string {
	path := "https://github.com/dunglas/mercure"
	r := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[\w.]+)?$`)
	if !r.MatchString(version) {
		return fmt.Sprintf("%s/releases/latest", path)
	}

	return fmt.Sprintf("%s/releases/tag/v%s", path, strings.TrimPrefix(version, "v"))
}
