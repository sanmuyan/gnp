package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"net"
	"reflect"
	"sync"
	"time"
)

type ProxyProvider interface {
	Close()
	Start()
}

type ProxyServer struct {
	*Server
	ctx          context.Context
	ctlMsg       *message.ControlMessage
	ctlConn      net.Conn
	userConnPool sync.Map
	tunnelConnCh chan *TunnelConn
}

func NewProxyServer(ctx context.Context, server *Server, ctlConn net.Conn, ctlMsg *message.ControlMessage) *ProxyServer {
	return &ProxyServer{
		ctx:          ctx,
		Server:       server,
		ctlMsg:       ctlMsg,
		ctlConn:      ctlConn,
		tunnelConnCh: make(chan *TunnelConn),
	}
}

func (p *ProxyServer) NewTunnel(sessionID string) {
	logrus.Infof("[%s] new request sessionID:=%s", p.ctlMsg.GetServiceID(), sessionID)
	msg := &message.ControlMessage{
		Ctl:       message.NewTunnel,
		Service:   p.ctlMsg.GetService(),
		ServiceID: p.ctlMsg.GetServiceID(),
		SessionID: sessionID,
	}
	err := p.Server.SendMsg(p.ctlConn, msg)
	if err != nil {
		logrus.Errorf("[%s] send ctl message %v", p.ctlMsg.GetServiceID(), err)
	}
}

func (p *ProxyServer) CleanUserConn() {
	// 清理超时的用户连接
	t := time.NewTicker(time.Second * time.Duration(p.Config.ConnTimeout))
	for range t.C {
		select {
		case <-p.ctx.Done():
			return
		default:
			p.userConnPool.Range(func(key, value any) bool {
				userConn, ok := value.(UserConnProvider)
				if !ok {
					logrus.Errorf("[%s] userConn type error %v sessionID:=%s", reflect.TypeOf(value), p.ctlMsg.GetServiceID(), key)
					return true
				}
				if !userConn.IsTunnelAvailable() {
					if userConn.GetCreateTime()+int64(p.Config.ConnTimeout) > time.Now().Unix() {
						p.RemoveUserConn(userConn.GetSessionID())
						logrus.Debugf("[%s] delete no tunnel userConn sessionID:=%s", p.ctlMsg.GetServiceID(), userConn.GetSessionID())
					}
				}
				return true
			})
		}
	}
}

func (p *ProxyServer) WatchTunnel() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case tunnelConn := <-p.tunnelConnCh:
			userConnProvider, ok := p.userConnPool.Load(tunnelConn.GetSessionID())
			if !ok {
				logrus.Errorf("[%s] user conn not exist %s", p.ctlMsg.GetServiceID(), tunnelConn.GetSessionID())
				continue
			}
			userConn := userConnProvider.(UserConnProvider)
			userConn.SetTunnelConn(tunnelConn)
			// 读取用户数据转发到隧道
			go userConn.TunnelToUser()
			// 读取隧道数据转发到用户
			go userConn.UserToTunnel()
		}
	}
}

func (p *ProxyServer) RemoveUserConn(sessionID string) {
	p.userConnPool.Delete(sessionID)
}
