package cmd

import (
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gnp/client"
	"gnp/pkg/config"
	"path"
	"runtime"
	"strings"
)

var cmdReady bool

var rootCmd = &cobra.Command{
	Use:   "gnps",
	Short: "GO NAT Proxy Server",
	Run: func(cmd *cobra.Command, args []string) {
		cmdReady = true
	},
	Example: "gnpc -c config.yaml\ngnpc --services tcp,127.0.0.1:3389,6100 --services udp,127.0.0.1:3389,6100 --server-host 127.0.0.1 --server-port 6000",
}

var configFile string

const (
	logLevel           = 4
	udpTunnelTimeOut   = 30
	keepAlivePeriod    = 2
	KeepAliveMaxFailed = 3
	serverHost         = "127.0.0.1"
	serverPort         = 6000
)

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "config file")
	rootCmd.Flags().IntP("log-level", "l", logLevel, "log level")
	rootCmd.Flags().StringP("server-host", "s", serverHost, "server bind address")
	rootCmd.Flags().IntP("server-port", "p", serverPort, "server bind port")
	rootCmd.Flags().String("password", "", "password")
	rootCmd.Flags().StringArray("services", nil, "services")
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
	viper.SetDefault("udp_tunnel_time_out", udpTunnelTimeOut)
	viper.SetDefault("keep_alive_period", keepAlivePeriod)
	viper.SetDefault("keep_alive_max_failed", KeepAliveMaxFailed)

	if len(configFile) > 0 {
		viper.SetConfigFile(configFile)
		err := viper.ReadInConfig()
		if err != nil {
			return err
		}
	}
	_ = viper.BindPFlag("log_level", rootCmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("server_host", rootCmd.Flags().Lookup("server-host"))
	_ = viper.BindPFlag("server_port", rootCmd.Flags().Lookup("server-port"))
	_ = viper.BindPFlag("password", rootCmd.Flags().Lookup("password"))

	err := viper.Unmarshal(&config.ServerConf)
	if err != nil {
		return err
	}

	services, err := rootCmd.Flags().GetStringArray("services")
	if err != nil {
		return err
	}

	for _, service := range services {
		parts := strings.Split(service, ",")
		if len(parts) == 3 {
			config.ServerConf.Services = append(config.ServerConf.Services, config.Service{
				Network:   parts[0],
				LocalAddr: parts[1],
				ProxyPort: parts[2],
			})
		}
	}

	logrus.SetLevel(logrus.Level(config.ServerConf.LogLevel))
	if logrus.Level(config.ServerConf.LogLevel) >= logrus.DebugLevel {
		logrus.SetReportCaller(true)
	}

	if len(config.ServerConf.ServerHost) == 0 {
		return errors.New("server host is empty")
	}

	if len(config.ServerConf.ServerPort) == 0 {
		return errors.New("server port is empty")
	}

	if len(config.ServerConf.Services) == 0 {
		return errors.New("services is empty")
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
		logrus.Debugf("config %+v", config.ServerConf)
		client.Run()
	}
}
