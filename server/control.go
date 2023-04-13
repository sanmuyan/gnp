package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/pkg/config"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
)

type Control struct {
	Config         config.ClientConfig
	userConnPool   map[string]chan net.Conn
	tunnelConnPool map[string]chan net.Conn
}

func NewControl(config config.ClientConfig) *Control {
	return &Control{
		Config:         config,
		userConnPool:   make(map[string]chan net.Conn),
		tunnelConnPool: make(map[string]chan net.Conn),
	}
}

func (c *Control) SendCtl(conn net.Conn, msg *message.Message, ctl int32) error {
	msg.Ctl = ctl
	_, err := conn.Write(msg.EncodeTCP())
	if err != nil {
		return err
	}
	return nil
}

func (c *Control) handelService(ctx context.Context, conn net.Conn, msg *message.Message) {
	if len(c.Config.Password) > 0 && !util.ComparePassword(msg.Auth, c.Config.Password) {
		logrus.Warnln("deny client", conn.RemoteAddr())
		return
	}
	logrus.Infoln("new service", msg.Service.Network, msg.Service.ProxyPort, conn.RemoteAddr())
	if !util.IsAllowPort(c.Config.AllowPorts, msg.Service.ProxyPort) {
		logrus.Warnln("deny service", msg.Service.ProxyPort)
		return
	}
	switch msg.Service.Network {
	case "tcp":
		tcpProxy := NewTCPProxy(ctx, c, msg, conn)
		c.tunnelConnPool[msg.SessionID] = tcpProxy.tunnelConnCh
		c.userConnPool[msg.SessionID] = tcpProxy.userConnCh
		err := c.SendCtl(conn, msg, message.ServiceReadyCtl)
		if err != nil {
			logrus.Errorf("send ctl message %+v %v", msg, err)
		}
		go tcpProxy.Start()
	case "udp":
		udpProxy := NewUDPProxy(ctx, c, msg, conn)
		err := c.SendCtl(conn, msg, message.ServiceReadyCtl)
		if err != nil {
			logrus.Errorf("send ctl message %+v %v", msg, err)
		}
		go udpProxy.Start()
	}
}

func (c *Control) controller(conn net.Conn) {
	// 处理客户端控制消息
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	message.ReadMessageTCP(conn, func(msg *message.Message, err error) (exit bool) {
		if err != nil {
			logrus.Debugln("read control message", err)
			_ = conn.Close()
			return true
		}
		logrus.Tracef("control message %+v client %s", msg, conn.RemoteAddr())
		switch msg.GetCtl() {
		case message.NewServiceCtl:
			// 处理客户端服务代理注册
			_ = util.SetReadDeadline(conn)(c.Config.ClientTimeOut)
			c.handelService(ctx, conn, msg)
		case message.NewTunnelCtl:
			// TCP隧道连接加入对应服务队列
			c.tunnelConnPool[msg.SessionID] <- conn
			return true
		case message.KeepAliveCtl:
			// 处理客户端会话保持
			_ = util.SetReadDeadline(conn)(c.Config.ClientTimeOut)
			_ = c.SendCtl(conn, msg, message.KeepAliveCtl)
		case message.LoginCtl:
			if len(c.Config.Password) > 0 && !util.ComparePassword(msg.Auth, c.Config.Password) {
				logrus.Warnln("deny client", conn.RemoteAddr())
				_ = conn.Close()
				return
			}
			msg.Auth = util.CreatePassword(c.Config.Password)
			_ = c.SendCtl(conn, msg, message.LoginCtl)
		}
		return false
	})
}

func (c *Control) handelControlConn(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logrus.Errorln("accept server connect", err)
			return
		}
		go c.controller(conn)
	}
}

func (c *Control) start() {
	listener, err := util.CreateListenTCP(c.Config.ServerBind, c.Config.ServerPort)
	if err != nil {
		logrus.Fatalln("server listen", err)
	}
	logrus.Infoln("server running", net.JoinHostPort(c.Config.ServerBind, c.Config.ServerPort))
	c.handelControlConn(listener)
}

func Run() {
	s := NewControl(config.ClientConf)
	go s.start()
	select {}
}
