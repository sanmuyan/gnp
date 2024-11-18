package server

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
)

// UDPProxy 处理 UDP 代理
type UDPProxy struct {
	*ProxyServer
	// conn 监听代理端口，处理用户连接
	conn *net.UDPConn
	// tunnelData 接收隧道数据的队列
	tunnelData chan *TunnelData
	// tunnelConn UDP 隧道数据连接
	tunnelConn *net.UDPConn
}

func NewUDPProxy(proxyServer *ProxyServer, tunnelConn *net.UDPConn) *UDPProxy {
	return &UDPProxy{
		ProxyServer: proxyServer,
		tunnelData:  make(chan *TunnelData, 10),
		tunnelConn:  tunnelConn,
	}
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
	go p.WatchTunnelData()
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
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.Errorf("[%s] read sessionID proxy %v", p.ctlMsg.GetServiceID(), err)
			return
		}
		go p.controller(buf[:n], remoteAddr)
	}
}

// controller 处理用户连接
func (p *UDPProxy) controller(data []byte, remoteAddr *net.UDPAddr) {
	// 判断是否存在用户连接，如果存在直接转发数据到隧道
	sessionID := remoteAddr.String()
	if userConn, ok := p.userConnPool.Load(sessionID); ok {
		p.handelUserConn(data, userConn.(*UDPUserConn))
		return
	}
	// 如果不存在，把用户连接存入用户连接池，然后通知客户端新建隧道连接
	cxt, cancel := context.WithCancel(p.ctx)
	userConn := NewUDPUserConn(NewUserConn(cxt, cancel, p.ProxyServer, sessionID), p.conn, p.tunnelConn, remoteAddr)
	p.userConnPool.Store(sessionID, userConn)
	p.NewTunnel(userConn.GetSessionID())

	// 设置连接池超时
	go userConn.waitTimeout()
	userConn.ResetTimeout()

	p.handelUserConn(data, userConn)
}

func (p *UDPProxy) WatchTunnelData() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case data := <-p.tunnelData:
			userConn, ok := p.userConnPool.Load(data.dataMsg.GetSessionID())
			if !ok {
				logrus.Errorf("[%s] user conn not found sessionID:=%s", p.ctlMsg.GetServiceID(), data.dataMsg.GetSessionID())
				return
			}
			_userConn := userConn.(*UDPUserConn)
			if _userConn.GetSessionID() != data.dataMsg.GetSessionID() {
				logrus.Warnf("[%s] user sessionID:=%s, tunnel sessionID:=%s", p.ctlMsg.GetServiceID(), _userConn.GetSessionID(), data.dataMsg.GetSessionID())
				return
			}
			select {
			case <-_userConn.ctx.Done():
				return
			default:
				_userConn.tunnelCh <- data.dataMsg.Data
			}
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
