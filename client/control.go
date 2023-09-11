package client

import (
	"bufio"
	"context"
	"github.com/sirupsen/logrus"
	"gnp/pkg/config"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"io"
	"net"
	"time"
)

type Client struct {
	ctx         context.Context
	cancel      context.CancelFunc
	Config      config.ClientConfig
	ctlConn     net.Conn
	keepAliveCh chan struct{}
}

func NewClient(ctx context.Context, cancel context.CancelFunc, config config.ClientConfig, ctlConn net.Conn) *Client {
	return &Client{
		ctx:         ctx,
		cancel:      cancel,
		Config:      config,
		ctlConn:     ctlConn,
		keepAliveCh: make(chan struct{}),
	}
}

func (c *Client) keepAlive() {
	t := time.NewTicker(time.Second * time.Duration(c.Config.KeepAlivePeriod))
	defer t.Stop()
	defer c.cancel()
	var count int
	go func() {
		for range t.C {
			_ = c.sendMsg(&message.ControlMessage{
				Ctl: message.KeepAlive,
			})
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
				return
			}
		case <-c.keepAliveCh:
			count = 0
		}
	}
}

func (c *Client) registryService() {
	// 请求服务器注册代理服务
	for _, item := range c.Config.Services {
		msg := &message.ControlMessage{
			Ctl: message.NewService,
			Service: &message.Service{
				ProxyPort: item.ProxyPort,
				LocalAddr: item.LocalAddr,
				Network:   item.Network,
			},
			ServiceID: item.Network + item.ProxyPort,
		}
		err := c.sendMsg(msg)
		if err != nil {
			logrus.Errorf("[%s] send control message %v", msg.GetService(), err)
		}
		logrus.Infof("[%s] registry service %s", msg.GetServiceID(), item.LocalAddr)
	}
}

func (c *Client) auth(msg *message.ControlMessage) bool {
	if c.Config.Token == msg.GetToken() {
		return true
	}
	return false
}

func (c *Client) sendMsg(msg *message.ControlMessage) error {
	msg.Token = c.Config.Token
	return message.WriteTCP(msg, c.ctlConn)
}

func (c *Client) controller() {
	// 处理服务端控制消息
	reader := bufio.NewReaderSize(c.ctlConn, message.ReadBufferSize)
	for {
		select {
		case <-c.ctx.Done():
			logrus.Infof("control exit, %v", c.ctx.Err())
			return
		default:
			msg, err := message.ReadTCP(reader)
			if err != nil {
				if err != io.EOF {
					logrus.Infof("server connect closed %v", err)
				} else {
					logrus.Errorf("read ctl message %v", err)
				}
				return
			}
			if !c.auth(msg) {
				logrus.Warnf("auth failed server=%s", c.ctlConn.RemoteAddr().String())
				continue
			}
			switch msg.GetCtl() {
			case message.ServiceReady:
				logrus.Infof("[%s] registry service ready", msg.GetServiceID())
			case message.NewTunnel:
				switch msg.GetService().GetNetwork() {
				case "tcp":
					go NewTCPTunnel(NewTunnel(c, msg)).NewTunnel()
				case "udp":
					go NewUDPTunnel(NewTunnel(c, msg)).NewTunnel()
				}
			case message.KeepAlive:
				c.keepAliveCh <- struct{}{}
			}
		}
	}
}

func start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := net.JoinHostPort(config.ClientConf.ServerHost, config.ClientConf.ServerPort)
	conn, err := util.CreateDialTCP(addr)
	if err != nil {
		logrus.Errorf("connect server %s %v", addr, err)
		return
	}
	client := NewClient(ctx, cancel, config.ClientConf, conn)
	go client.registryService()
	go client.keepAlive()
	client.controller()
}

func Run() {
	for {
		logrus.Infof("connect server %s:%s", config.ClientConf.ServerHost, config.ClientConf.ServerPort)
		start()
		time.Sleep(time.Second)
	}
}
