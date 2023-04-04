package config

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"path"
	"runtime"
)

const defaultConfig = `
log_level: 4
server:
  server_bind: 0.0.0.0
  server_port: 6000
  allow_ports: 0-65535
  udp_tunnel_time_out: 30
  client_time_out: 60
client:
  keep_alive_period: 2
  keep_alive_max_failed: 3
  udp_tunnel_time_out: 30
`

type Service struct {
	ProxyPort string `yaml:"proxy_port"`
	LocalAddr string `yaml:"local_addr"`
	Network   string `yaml:"network"`
}

type Client struct {
	ServerHost         string    `yaml:"server_host"`
	ServerPort         string    `yaml:"server_port"`
	Services           []Service `yaml:"services"`
	Password           string    `yaml:"password"`
	KeepAlivePeriod    int       `yaml:"keep_alive_period"`
	KeepAliveMaxFailed int       `yaml:"keep_alive_max_failed"`
	UDPTunnelTimeOut   int       `yaml:"udp_tunnel_time_out"`
}

type Server struct {
	ServerBind       string `yaml:"server_bind"`
	ServerPort       string `yaml:"server_port"`
	Password         string `yaml:"password"`
	AllowPorts       string `yaml:"allow_ports"`
	UDPTunnelTimeOut int    `yaml:"udp_tunnel_time_out"`
	ClientTimeOut    int    `yaml:"client_time_out"`
}

type Config struct {
	LogLevel int    `yaml:"log_level"`
	Client   Client `yaml:"client"`
	Server   Server `yaml:"server"`
}

func NewConfig(configFile string) *Config {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			fileName := path.Base(frame.File)
			return frame.Function, fileName
		},
	})

	configByte, err := ioutil.ReadFile(configFile)
	if err != nil {
		logrus.Fatalln("config", err)
	}
	config := &Config{}
	err = yaml.Unmarshal([]byte(defaultConfig), config)
	if err != nil {
		logrus.Fatalln("config", err)
	}

	if err := yaml.Unmarshal(configByte, config); err != nil {
		logrus.Fatalln("config", err)
	}

	logrus.SetLevel(logrus.Level(config.LogLevel))
	if logrus.Level(config.LogLevel) >= logrus.DebugLevel {
		logrus.SetReportCaller(true)
	}
	logrus.Debugf("config %+v", config)
	return config
}
