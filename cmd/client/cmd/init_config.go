package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sanmuyan/xpkg/xutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gnp/pkg/config"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func initConfig(cmd *cobra.Command) error {
	// 设置日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			fileName := path.Base(frame.File)
			return frame.Function, fileName
		},
	})

	viper.SetConfigName("config")
	// 配置文件和命令行参数都不指定时的默认配置
	viper.SetDefault("conn_timeout", connTimout)
	viper.SetDefault("keep_alive_period", keepAlivePeriod)
	viper.SetDefault("keep_alive_max_failed", KeepAliveMaxFailed)

	// 设置默认配置文件
	if len(configFile) == 0 {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		configFile = dir + "/config.yaml"
		configFile = filepath.Clean(configFile)
		if _, err := os.Stat(configFile); err != nil {
			configFile = ""
		}
	}

	// 读取配置文件
	if len(configFile) > 0 {
		viper.SetConfigFile(configFile)
		err := viper.ReadInConfig()
		if err != nil {
			return err
		}
	}

	// 绑定命令行参数到配置项
	// 配置项优先级：命令行参数 > 配置文件 > 默认命令行参数
	_ = viper.BindPFlags(cmd.Flags())
	_ = viper.BindPFlag("log_level", cmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("server_host", cmd.Flags().Lookup("server-host"))
	_ = viper.BindPFlag("server_port", cmd.Flags().Lookup("server-port"))
	_ = viper.BindPFlag("token", cmd.Flags().Lookup("token"))

	err := viper.Unmarshal(&config.ClientConf)
	if err != nil {
		return err
	}

	services, err := cmd.Flags().GetStringArray("services")
	if err != nil {
		return err
	}

	for _, service := range services {
		parts := strings.Split(service, ",")
		if len(parts) == 3 {
			config.ClientConf.Services = append(config.ClientConf.Services, config.Service{
				Network:   parts[0],
				LocalAddr: parts[1],
				ProxyPort: parts[2],
			})
		}
	}

	logrus.SetLevel(logrus.Level(config.ClientConf.LogLevel))
	if logrus.Level(config.ClientConf.LogLevel) >= logrus.DebugLevel {
		logrus.SetReportCaller(true)
	}

	if len(configFile) == 0 {
		logrus.Warn("no specified config file, maybe should use '-c config.yaml' flag")
	}

	if viper.GetBool("pprof-server") {
		go pprofServer(7777)
	}

	if len(config.ClientConf.ServerHost) == 0 {
		return errors.New("server host is empty")
	}

	if len(config.ClientConf.ServerPort) == 0 {
		return errors.New("server port is empty")
	}

	if len(config.ClientConf.Services) == 0 {
		return errors.New("services is empty")
	}
	logrus.Debugf("config init completed: %+v", string(xutil.RemoveError(json.Marshal(config.ClientConf))))
	return nil
}

func pprofServer(port int) {
	logrus.Infof("pprof server listening on 0.0.0.0:%d", port)
	err := http.ListenAndServe("0.0.0.0:"+fmt.Sprintf("%d", port), nil)
	if err != nil {
		logrus.Warnf("pprof server error: %s", err)
		pprofServer(port + 1)
	}
}
