package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/pkg/util"
	"net"
)

type TCPProxy struct {
	*ProxyServer
	listener net.Listener
}

func NewTCPProxy(proxyServer *ProxyServer) *TCPProxy {
	return &TCPProxy{ProxyServer: proxyServer}
}

func (p *TCPProxy) Start() {
	listener, err := util.CreateListenTCP(p.Config.ServerBind, p.ctlMsg.Service.ProxyPort)
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
			logrus.Debugf("[%s] accept proxy connect %s", p.ctlMsg.ServiceID, err)
			return
		}
		go p.handelUserConn(conn)
	}
}

func (p *TCPProxy) handelUserConn(conn net.Conn) {
	ctx, cancel := context.WithCancel(p.ctx)
	userConn := NewTCPUserConn(NewUserConn(ctx, cancel, p.ProxyServer, conn.RemoteAddr().String()), conn)
	p.userConnPool.Store(userConn.GetSessionID(), userConn)
	p.NewTunnel(userConn.GetSessionID())
	userConn.ResetTimeout()
}
