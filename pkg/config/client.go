package config

type Service struct {
	ProxyPort string `mapstructure:"proxy_port"`
	LocalAddr string `mapstructure:"local_addr"`
	Network   string `mapstructure:"network"`
}

type ClientConfig struct {
	LogLevel           int       `mapstructure:"log_level"`
	ServerHost         string    `mapstructure:"server_host"`
	ServerPort         string    `mapstructure:"server_port"`
	Services           []Service `mapstructure:"services"`
	Token              string    `mapstructure:"token"`
	KeepAlivePeriod    int       `mapstructure:"keep_alive_period"`
	KeepAliveMaxFailed int       `mapstructure:"keep_alive_max_failed"`
	ConnTimeout        int       `mapstructure:"conn_timeout"`
}

var ClientConf ClientConfig
