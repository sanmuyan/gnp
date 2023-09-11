package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
)

type UDPProxy struct {
	*ProxyServer
	conn *net.UDPConn
}

func NewUDPProxy(proxyServer *ProxyServer) *UDPProxy {
	return &UDPProxy{ProxyServer: proxyServer}
}

func (p *UDPProxy) Start() {
	conn, err := util.CreateListenUDP(p.Config.ServerBind, p.ctlMsg.GetService().GetProxyPort())
	if err != nil {
		logrus.Errorf("[%s] proxy listen %v", p.ctlMsg.GetServiceID(), err)
		return
	}
	p.conn = conn
	defer p.Close()
	p.connSet()
	go p.CleanUserConn()
	go p.WatchTunnel()
	go p.handleConn()
	<-p.ctx.Done()
}

func (p *UDPProxy) Close() {
	_ = p.conn.Close()
	p.Server.Clean(p.ctlMsg.GetServiceID())
	logrus.Infof("[%s] close service", p.ctlMsg.ServiceID)
}

func (p *UDPProxy) handleConn() {
	for {
		buf := make([]byte, message.BufDataSize)
		n, remoteAddr, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			logrus.Errorf("[%s] read sessionID proxy %v", p.ctlMsg.GetServiceID(), err)
			return
		}
		go p.controller(buf[:n], remoteAddr)
	}
}

func (p *UDPProxy) controller(data []byte, remoteAddr *net.UDPAddr) {
	sessionID := remoteAddr.String()
	if userConn, ok := p.userConnPool.Load(sessionID); ok {
		p.handelUserConn(data, userConn.(*UDPUserConn))
		return
	}
	msg, err := message.Unmarshal(data)
	if err != nil || msg.GetCtl() < 10000 || msg.GetServiceID() != p.ctlMsg.GetServiceID() {
		userConn, ok := p.userConnPool.Load(sessionID)
		if !ok {
			cxt, cancel := context.WithCancel(p.ctx)
			_userConn := NewUDPUserConn(NewUserConn(cxt, cancel, p.ProxyServer, sessionID), p.conn, remoteAddr)
			p.userConnPool.Store(sessionID, _userConn)
			p.NewTunnel(_userConn.GetSessionID())

			go _userConn.waitTimeout()
			_userConn.ResetTimeout()

			p.handelUserConn(data, _userConn)
			return
		}
		p.handelUserConn(data, userConn.(*UDPUserConn))
		return
	}
	p.handelTunnelConn(msg, remoteAddr)
}

func (p *UDPProxy) handelTunnelConn(msg *message.ControlMessage, remoteAddr *net.UDPAddr) {
	switch msg.GetCtl() {
	case message.NewTunnel:
		p.tunnelConnCh <- NewTunnelConn(nil, msg, remoteAddr)
	case message.NewDataConn:
		userConn, ok := p.userConnPool.Load(msg.GetSessionID())
		if !ok {
			logrus.Errorf("[%s] user conn not found sessionID:=%s", p.ctlMsg.GetServiceID(), msg.GetSessionID())
			return
		}
		_userConn := userConn.(*UDPUserConn)
		if _userConn.GetSessionID() != msg.GetSessionID() {
			logrus.Warnf("[%s] user sessionID:=%s, tunnel sessionID:=%s", p.ctlMsg.GetServiceID(), _userConn.GetSessionID(), msg.GetSessionID())
			return
		}
		select {
		case <-_userConn.ctx.Done():
			return
		default:
			_userConn.tunnelCh <- msg.Data
		}
	}
}

func (p *UDPProxy) handelUserConn(data []byte, userConn *UDPUserConn) {
	select {
	case <-userConn.ctx.Done():
		return
	default:
		userConn.userCh <- data
	}
}

func (p *UDPProxy) connSet() {
	_ = p.conn.SetWriteBuffer(message.WriteBufferSize)
}
