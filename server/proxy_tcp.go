package server

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"gnp/pkg/util"
	"net"
)

// TCPProxy 处理 TCP 代理
type TCPProxy struct {
	*ProxyServer
	// listener 监听代理端口，处理用户连接
	listener net.Listener
}

func NewTCPProxy(proxyServer *ProxyServer) *TCPProxy {
	return &TCPProxy{ProxyServer: proxyServer}
}

func (p *TCPProxy) Start() {
	listener, err := util.CreateListenTCP(p.Config.ServerBind, p.ctlMsg.Service.ProxyPort)
	if errors.Is(err, net.ErrClosed) {
		return
	}
	if err != nil {
		logrus.Errorf("[%s] proxy listen %v", p.ctlMsg.GetServiceID(), err)
		return
	}
	p.listener = listener
	defer p.Close()
	go p.handelConn()
	go p.WatchTunnel()
	go p.CleanUserConn()
	<-p.ctx.Done()
}

func (p *TCPProxy) Close() {
	p.Server.Clean(p.ctlMsg.GetServiceID())
	_ = p.listener.Close()
	logrus.Infof("[%s] close service", p.ctlMsg.ServiceID)
}

func (p *TCPProxy) handelConn() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.Errorf("[%s] accept proxy connect %s", p.ctlMsg.ServiceID, err)
			return
		}
		go p.controller(conn)
	}
}

// controller 处理用户连接
func (p *TCPProxy) controller(conn net.Conn) {
	// 把用户连接存入用户连接池
	ctx, cancel := context.WithCancel(p.ctx)
	userConn := NewTCPUserConn(NewUserConn(ctx, cancel, p.ProxyServer, conn.RemoteAddr().String()), conn)
	p.userConnPool.Store(userConn.GetSessionID(), userConn)
	// 通知客户端新建隧道
	p.NewTunnel(userConn.GetSessionID())
	// 设置连接池超时
	userConn.ResetTimeout()
}
