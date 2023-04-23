package client

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"gnp/pkg/config"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
	"time"
)

type Control struct {
	ctlConn     net.Conn
	Config      config.ServerConfig
	done        chan bool
	keepAliveCh chan bool
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewControl(config config.ServerConfig) *Control {
	return &Control{
		Config:      config,
		done:        make(chan bool),
		keepAliveCh: make(chan bool),
	}
}
func (c *Control) registryService() {
	// 请求服务器注册代理服务
	for _, item := range c.Config.Services {
		err := c.SendCtl(c.ctlConn, message.NewMessage(&message.Options{
			Auth: util.CreatePassword(c.Config.Password),
			Service: &message.Service{
				ProxyPort: item.ProxyPort,
				LocalAddr: item.LocalAddr,
				Network:   item.Network,
			},
			SessionID: item.Network + item.ProxyPort,
		}), message.NewServiceCtl)
		if err != nil {
			logrus.Errorln("write control message", err)
		}
		logrus.Infoln("registry service", item.Network, net.JoinHostPort(c.Config.ServerHost, item.ProxyPort), item.LocalAddr)
	}
}

func (c *Control) keepAlive() {
	t := time.NewTicker(time.Second * time.Duration(c.Config.KeepAlivePeriod))
	defer t.Stop()
	var count int
	go func() {
		for range t.C {
			_ = c.SendCtl(c.ctlConn, message.NewMessage(&message.Options{}), message.KeepAliveCtl)
		}
	}()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(time.Second * time.Duration(c.Config.KeepAlivePeriod+1)):
			count += 1
			if count > c.Config.KeepAliveMaxFailed {
				logrus.Errorln("keep alive max timeout")
				c.cancel()
				return
			}
		case <-c.keepAliveCh:
			count = 0
		}
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

func (c *Control) controller(msg *message.Message) {
	// 处理服务端控制消息
	switch msg.GetCtl() {
	case message.ServiceReadyCtl:
		logrus.Infoln("service ready", msg.Service.Network, msg.Service.ProxyPort)
	case message.NewTunnelCtl:
		// 新建隧道连接
		switch msg.Service.Network {
		case "tcp":
			NewTCPTunnel(c, msg).Process()
		case "udp":
			NewUDPTunnel(c, msg).Process()
		}
	case message.KeepAliveCtl:
		c.keepAliveCh <- true
	}
}

func (c *Control) handelConn() {
	defer func() {
		_ = c.ctlConn.Close()
		c.cancel()
	}()
	logrus.Infoln("connect server", net.JoinHostPort(c.Config.ServerHost, c.Config.ServerPort))
	message.ReadMessageTCP(c.ctlConn, func(msg *message.Message, err error) (exit bool) {
		if err != nil {
			logrus.Debugln("read control message", err)
			return true
		}
		logrus.Tracef("control message %+v", msg)
		go c.controller(msg)
		return false
	})
}

func (c *Control) close() {
	c.done <- true
}

func (c *Control) login() error {
	sendAuth := util.CreatePassword(c.Config.Password)
	_, err := c.ctlConn.Write(message.NewMessage(&message.Options{
		Ctl:  message.LoginCtl,
		Auth: sendAuth,
	}).EncodeTCP())
	if err != nil {
		return err
	}
	res := make(chan error)
	go func() {
		message.ReadMessageTCP(c.ctlConn, func(msg *message.Message, err error) (exit bool) {
			if err != nil {
				res <- err
				return true
			}
			if msg != nil {
				if msg.GetCtl() == message.LoginCtl {
					if len(c.Config.Password) > 0 {
						if msg.Auth == sendAuth || !util.ComparePassword(msg.Auth, c.Config.Password) {
							res <- errors.New("deny server")
						}
					}
					res <- nil
				}
			}
			return true
		})
	}()
	select {
	case <-time.After(time.Second * time.Duration(c.Config.KeepAlivePeriod)):
	case err = <-res:
		return err
	}
	return errors.New("timeout")
}

func (c *Control) start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.cancel = cancel
	defer c.close()
	conn, err := util.CreateDialTCP(net.JoinHostPort(c.Config.ServerHost, c.Config.ServerPort))
	if err != nil {
		logrus.Errorln("server connect", err)
		return
	}
	c.ctlConn = conn
	if err := c.login(); err != nil {
		logrus.Errorln("login", err)
		_ = c.ctlConn.Close()
		return
	}
	go c.keepAlive()
	go c.registryService()
	go c.handelConn()
	<-ctx.Done()
}

func Run() {
	c := NewControl(config.ServerConf)
	go c.start()
	for {
		select {
		case <-c.done:
			logrus.Infoln("retry connect server", net.JoinHostPort(c.Config.ServerHost, c.Config.ServerPort))
			time.Sleep(time.Second * 1)
			go c.start()
		}
	}
}
