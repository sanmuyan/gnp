package cmd

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gnp/client"
)

var rootCtx context.Context

var rootCmd = &cobra.Command{
	Use:   "gnps",
	Short: "GO NAT Proxy Server",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		err := initConfig(cmd)
		if err != nil {
			logrus.Fatalf("init config error: %v", err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client.Run(rootCtx)
	},
	Example: "gnpc --services tcp,127.0.0.1:3389,6100 --services udp,127.0.0.1:3389,6100 -s localhost -p 6000",
}

var configFile string

const (
	logLevel           = 4
	connTimout         = 3600
	keepAlivePeriod    = 2
	KeepAliveMaxFailed = 3
	serverHost         = "localhost"
	serverPort         = 6000
)

func init() {
	// 初始化命令行参数
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file")
	rootCmd.PersistentFlags().IntP("log-level", "l", logLevel, "log level")
	rootCmd.PersistentFlags().BoolP("pprof-server", "", false, "enable pprof server")
	rootCmd.Flags().StringP("server-host", "s", serverHost, "server bind address")
	rootCmd.Flags().IntP("server-port", "p", serverPort, "server bind port")
	rootCmd.Flags().String("token", "", "token")
	rootCmd.Flags().StringArray("services", nil, "services")
}

func Execute(ctx context.Context) {
	rootCtx = ctx
	if err := rootCmd.Execute(); err != nil {
		logrus.Tracef("cmd execute error: %v", err)
	}
}
