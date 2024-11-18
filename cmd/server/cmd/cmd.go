package cmd

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gnp/pkg/config"
	"gnp/server"
	"log"
	"net/http"
	_ "net/http/pprof"
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
	logLevel    = 4
	serverBind  = "0.0.0.0"
	serverPort  = 6000
	allowPorts  = "1-65535"
	connTimeout = 3600
)

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "config file")
	rootCmd.Flags().IntP("log-level", "l", logLevel, "log level")
	rootCmd.Flags().StringP("server-bind", "s", serverBind, "server bind address")
	rootCmd.Flags().IntP("server-port", "p", serverPort, "server bind port")
	rootCmd.Flags().String("token", "", "token")
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
	viper.SetDefault("conn_timeout", connTimeout)

	if len(configFile) > 0 {
		viper.SetConfigFile(configFile)
		err := viper.ReadInConfig()
		if err != nil {
			return err
		}
	}

	_ = viper.BindPFlag("log_level", rootCmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("server_bind", rootCmd.Flags().Lookup("server-bind"))
	_ = viper.BindPFlag("server_port", rootCmd.Flags().Lookup("server-port"))
	_ = viper.BindPFlag("token", rootCmd.Flags().Lookup("token"))
	_ = viper.BindPFlag("allow_ports", rootCmd.Flags().Lookup("allow-ports"))

	err := viper.Unmarshal(&config.ServerConf)
	if err != nil {
		return err
	}
	logrus.SetLevel(logrus.Level(config.ServerConf.LogLevel))
	if logrus.Level(config.ServerConf.LogLevel) >= logrus.DebugLevel {
		go func() {
			err := http.ListenAndServe("0.0.0.0:7777", nil)
			if err != nil {
				log.Fatalf("Debug error: %v", err)
			}
		}()
		logrus.SetReportCaller(true)
	}
	return nil
}

func Execute(ctx context.Context) {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
	if cmdReady {
		err := initConfig()
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Debugf("config %+v", config.ServerConf)
		server.Run(ctx)
	}
}
