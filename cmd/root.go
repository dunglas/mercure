package cmd

import (
	"os"

	fluentd "github.com/joonix/log"
	log "github.com/sirupsen/logrus"

	"github.com/dunglas/mercure/hub"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mercure",
	Short: "Start the Mercure Hub",
	Long: `Mercure is a protocol allowing to push data updates to web browsers and
other HTTP clients in a convenient, fast, reliable and battery-efficient way.
The Mercure Hub is the reference implementation of the Mercure protocol.

Go to https://mercure.rocks for more information!`,
	Run: func(cmd *cobra.Command, args []string) {
		hub, err := hub.NewHub(viper.GetViper())
		if err != nil {
			log.Fatalln(err)
		}

		defer func() {
			if err = hub.Stop(); err != nil {
				log.Fatalln(err)
			}
		}()

		hub.Serve()
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
	fs := rootCmd.Flags()

	fs.BoolP("debug", "d", false, "enable the debug mode")
	viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))

	fs.StringP("transport-url", "t", "", "transport and history system to use")
	viper.BindPFlag("transport_url", rootCmd.Flags().Lookup("transport-url"))

	fs.StringP("jwt-key", "k", "", "JWT key")
	viper.BindPFlag("jwt_key", rootCmd.Flags().Lookup("jwt-key"))

	fs.StringP("jwt-algorithm", "O", "", "JWT algorithm")
	viper.BindPFlag("jwt_algorithm", rootCmd.Flags().Lookup("jwt-algorithm"))

	fs.StringP("publisher-jwt-key", "K", "", "publisher JWT key")
	viper.BindPFlag("publisher_jwt_key", rootCmd.Flags().Lookup("publisher-jwt-key"))

	fs.StringP("publisher-jwt-algorithm", "A", "", "publisher JWT algorithm")
	viper.BindPFlag("publisher_jwt_algorithm", rootCmd.Flags().Lookup("publisher-jwt-algorithm"))

	fs.StringP("subscriber-jwt-key", "L", "", "subscriber JWT key")
	viper.BindPFlag("subscriber_jwt_key", rootCmd.Flags().Lookup("subscriber-jwt-key"))

	fs.StringP("subscriber-jwt-algorithm", "B", "", "subscriber JWT algorithm")
	viper.BindPFlag("subscriber_jwt_algorithm", rootCmd.Flags().Lookup("subscriber-jwt-algorithm"))

	fs.BoolP("allow-anonymous", "X", false, "allow subscribers with no valid JWT to connect")
	viper.BindPFlag("allow_anonymous", rootCmd.Flags().Lookup("allow-anonymous"))

	fs.StringSliceP("cors-allowed-origins", "c", []string{}, "list of allowed CORS origins")
	viper.BindPFlag("cors_allowed_origins", rootCmd.Flags().Lookup("cors-allowed-origins"))

	fs.StringSliceP("publish-allowed-origins", "p", []string{}, "list of origins allowed to publish")
	viper.BindPFlag("publish_allowed_origins", rootCmd.Flags().Lookup("publish-allowed-origins"))

	fs.StringP("addr", "a", "", "the address to listen on")
	viper.BindPFlag("addr", rootCmd.Flags().Lookup("addr"))

	fs.StringSliceP("acme-hosts", "o", []string{}, "list of hosts for which Let's Encrypt certificates must be issued")
	viper.BindPFlag("acme_hosts", rootCmd.Flags().Lookup("acme-hosts"))

	fs.StringP("acme-cert-dir", "E", "", "the directory where to store Let's Encrypt certificates")
	viper.BindPFlag("acme_cert_dir", rootCmd.Flags().Lookup("acme-cert-dir"))

	fs.StringP("cert-file", "C", "", "a cert file (to use a custom certificate)")
	viper.BindPFlag("cert_file", rootCmd.Flags().Lookup("cert-file"))

	fs.StringP("key-file", "J", "", "a key file (to use a custom certificate)")
	viper.BindPFlag("key_file", rootCmd.Flags().Lookup("key-file"))

	fs.StringP("heartbeat-interval", "i", "", "interval between heartbeats (0s to disable)")
	viper.BindPFlag("heartbeat_interval", rootCmd.Flags().Lookup("heartbeat-interval"))

	fs.StringP("read-timeout", "R", "", "maximum duration for reading the entire request, including the body")
	viper.BindPFlag("read_timeout", rootCmd.Flags().Lookup("read-timeout"))

	fs.StringP("write-timeout", "W", "", "maximum duration before timing out writes of the response")
	viper.BindPFlag("write_timeout", rootCmd.Flags().Lookup("write-timeout"))

	fs.BoolP("compress", "Z", false, "enable or disable HTTP compression support")
	viper.BindPFlag("compress", rootCmd.Flags().Lookup("compress"))

	fs.BoolP("use-forwarded-headers", "f", false, "enable headers forwarding")
	viper.BindPFlag("use_forwarded_headers", rootCmd.Flags().Lookup("use-forwarded-headers"))

	fs.BoolP("demo", "D", false, "enable the demo mode")
	viper.BindPFlag("demo", rootCmd.Flags().Lookup("demo"))

	fs.StringP("log-format", "l", "", "the log format (JSON, FLUENTD or TEXT)")
	viper.BindPFlag("log_format", rootCmd.Flags().Lookup("log-format"))

	cobra.OnInitialize(initConfig)

	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	switch viper.GetString("log_format") {
	case "JSON":
		log.SetFormatter(&log.JSONFormatter{})
		break
	case "FLUENTD":
		log.SetFormatter(fluentd.NewFormatter())
		break
	}

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	v := viper.GetViper()
	hub.SetConfigDefaults(v)

	v.SetConfigName("mercure")
	v.AutomaticEnv()

	v.AddConfigPath(".")
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = "$HOME/.config"
	}
	v.AddConfigPath(configDir + "/mercure/")
	v.AddConfigPath("/etc/mercure/")

	v.ReadInConfig()
}
