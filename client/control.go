package client

import (
	"bufio"
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"gnp/pkg/config"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"io"
	"net"
	"time"
)

// Client 客户端控制中心
type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
	Config config.ClientConfig
	// ctlConn 客户端控制连接
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

// keepAlive 定时向服务端发送心跳，如果服务端没有响应则关闭客户端控制并重新连接
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

// registryService 请求服务器注册代理服务
func (c *Client) registryService() {
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

// controller 处理服务端控制消息
func (c *Client) controller() {
	defer func() {
		logrus.Info("control closed")
	}()
	reader := bufio.NewReaderSize(c.ctlConn, message.ReadBufferSize)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			msg, err := message.ReadTCP(reader)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
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
				logrus.Infof("[%s] new tunnel sessionID:=%s", msg.GetServiceID(), msg.GetSessionID())
				switch msg.GetService().GetNetwork() {
				case "tcp":
					go NewTCPTunnel(NewTunnel(c.ctx, c, msg)).NewTunnel()
				case "udp":
					go NewUDPTunnel(NewTunnel(c.ctx, c, msg)).NewTunnel()
				}
			case message.KeepAlive:
				c.keepAliveCh <- struct{}{}
			default:
				logrus.Warnf("[%s] unknown ctl:=%d", msg.GetServiceID(), msg.GetCtl())
			}
		}
	}
}

func start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
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
	go client.controller()
	defer func() {
		_ = conn.Close()
	}()
	<-ctx.Done()
}

func Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			logrus.Infof("connect server %s:%s", config.ClientConf.ServerHost, config.ClientConf.ServerPort)
			start(ctx)
			time.Sleep(time.Second)
		}
	}
}
