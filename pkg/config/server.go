package config

type ServerConfig struct {
	LogLevel    int    `mapstructure:"log_level"`
	ServerBind  string `mapstructure:"server_bind"`
	ServerPort  string `mapstructure:"server_port"`
	Token       string `mapstructure:"token"`
	AllowPorts  string `mapstructure:"allow_ports"`
	ConnTimeout int    `mapstructure:"conn_timeout"`
}

var ServerConf ServerConfig
