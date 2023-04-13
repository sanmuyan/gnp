package config

type ClientConfig struct {
	LogLevel         int    `mapstructure:"log_level"`
	ServerBind       string `mapstructure:"server_bind"`
	ServerPort       string `mapstructure:"server_port"`
	Password         string `mapstructure:"password"`
	AllowPorts       string `mapstructure:"allow_ports"`
	UDPTunnelTimeOut int    `mapstructure:"udp_tunnel_time_out"`
	ClientTimeOut    int    `mapstructure:"client_time_out"`
}

var ClientConf ClientConfig
