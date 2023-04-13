package config

type Service struct {
	ProxyPort string `mapstructure:"proxy_port"`
	LocalAddr string `mapstructure:"local_addr"`
	Network   string `mapstructure:"network"`
}

type Config struct {
	LogLevel           int       `mapstructure:"log_level"`
	ServerHost         string    `mapstructure:"server_host"`
	ServerPort         string    `mapstructure:"server_port"`
	Services           []Service `mapstructure:"services"`
	Password           string    `mapstructure:"password"`
	KeepAlivePeriod    int       `mapstructure:"keep_alive_period"`
	KeepAliveMaxFailed int       `mapstructure:"keep_alive_max_failed"`
	UDPTunnelTimeOut   int       `mapstructure:"udp_tunnel_time_out"`
}

var Conf Config
