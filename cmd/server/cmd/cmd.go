package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gnp/pkg/config"
	"gnp/server"
	"path"
	"runtime"
)

var cmdReady bool

var rootCmd = &cobra.Command{
	Use:   "gnps",
	Short: "GO NAT Proxy Server",
	Run: func(cmd *cobra.Command, args []string) {
		cmdReady = true
	},
	Example: "gnps -c config.yaml\ngnps --server-bind 0.0.0.0 --server-port 6000",
}

var configFile string

const (
	logLevel         = 4
	serverBind       = "0.0.0.0"
	serverPort       = 6000
	allowPorts       = "1-65535"
	udpTunnelTimeOut = 30
	clientTimeOut    = 30
)

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "config file")
	rootCmd.Flags().IntP("log-level", "l", logLevel, "log level")
	rootCmd.Flags().StringP("server-bind", "s", serverBind, "server bind address")
	rootCmd.Flags().IntP("server-port", "p", serverPort, "server bind port")
	rootCmd.Flags().String("password", "", "password")
	rootCmd.Flags().String("allow-ports", allowPorts, "allow ports")
}

func initConfig() error {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			fileName := path.Base(frame.File)
			return frame.Function, fileName
		},
	})

	viper.SetConfigName("config")
	viper.SetDefault("log_level", logLevel)
	viper.SetDefault("server_bind", serverBind)
	viper.SetDefault("server_port", serverPort)
	viper.SetDefault("allow_ports", allowPorts)
	viper.SetDefault("udp_tunnel_time_out", udpTunnelTimeOut)
	viper.SetDefault("client_time_out", clientTimeOut)

	if len(configFile) > 0 {
		viper.SetConfigFile(configFile)
		err := viper.ReadInConfig()
		if err != nil {
			return err
		}
	} else {
		_ = viper.BindPFlag("log_level", rootCmd.Flags().Lookup("log-level"))
		_ = viper.BindPFlag("server_bind", rootCmd.Flags().Lookup("server-bind"))
		_ = viper.BindPFlag("server_port", rootCmd.Flags().Lookup("server-port"))
		_ = viper.BindPFlag("password", rootCmd.Flags().Lookup("password"))
		_ = viper.BindPFlag("allow_ports", rootCmd.Flags().Lookup("allow-ports"))
	}

	err := viper.Unmarshal(&config.ClientConf)
	if err != nil {
		return err
	}
	logrus.SetLevel(logrus.Level(config.ClientConf.LogLevel))
	if logrus.Level(config.ClientConf.LogLevel) >= logrus.DebugLevel {
		logrus.SetReportCaller(true)
	}
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
	if cmdReady {
		err := initConfig()
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Debugf("config %+v", config.ClientConf)
		server.Run()
	}
}
