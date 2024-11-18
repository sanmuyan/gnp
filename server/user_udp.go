package server

import (
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"net"
	"sync"
	"time"
)

// UDPUserConn 转发用户 UDP 数据
type UDPUserConn struct {
	*UserConn
	// 代理端口连接
	conn *net.UDPConn
	// udpTunnelConn 隧道连接
	udpTunnelConn *net.UDPConn
	// userCh 用户数据队列
	userCh chan []byte
	// tunnelCh 隧道数据队列
	tunnelCh chan []byte
	// remoteAddr 用户 UDP 地址
	remoteAddr *net.UDPAddr
	// timeout 用户连接池超时时间，如果超时则从连接池中删除
	timeout sync.Map
}

func NewUDPUserConn(userConn *UserConn, conn, tunnelConn *net.UDPConn, remoteAddr *net.UDPAddr) *UDPUserConn {
	return &UDPUserConn{
		UserConn:      userConn,
		userCh:        make(chan []byte, 10),
		tunnelCh:      make(chan []byte, 10),
		conn:          conn,
		udpTunnelConn: tunnelConn,
		remoteAddr:    remoteAddr,
	}
}

func (u *UDPUserConn) UserToTunnel() {
	defer u.Close()
	for {
		select {
		case <-u.ctx.Done():
			return
		case data := <-u.userCh:
			err := message.WriteToUDP(&message.ControlMessage{
				Ctl:       message.NewTunnelData,
				ServiceID: u.proxyServer.ctlMsg.GetServiceID(),
				SessionID: u.GetSessionID(),
				Data:      data,
			}, u.udpTunnelConn, u.tunnelConn.remoteAddr)
			if err != nil {
				logrus.Tracef("[%s] write to tunnel %v", u.proxyServer.ctlMsg.GetServiceID(), err)
				return
			}
			u.ResetTimeout()
		}
	}
}

func (u *UDPUserConn) TunnelToUser() {
	defer u.Close()
	for {
		select {
		case <-u.ctx.Done():
			return
		case data := <-u.tunnelCh:
			_, err := u.conn.WriteToUDP(data, u.remoteAddr)
			if err != nil {
				logrus.Tracef("[%s] write to user %v", u.proxyServer.ctlMsg.GetServiceID(), err)
				return
			}
			u.ResetTimeout()
		}
	}
}

func (u *UDPUserConn) ResetTimeout() {
	u.timeout.Store(1, time.Now().Unix()+int64(u.proxyServer.Config.ConnTimeout))
}

func (u *UDPUserConn) waitTimeout() {
	defer u.Close()
	t := time.NewTicker(time.Second * time.Duration(u.proxyServer.Config.ConnTimeout))
	for range t.C {
		timeout, _ := u.timeout.Load(1)
		if time.Now().Unix() > timeout.(int64) {
			return
		}
	}
}
