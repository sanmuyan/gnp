package server

import (
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"net"
	"sync"
	"time"
)

type UDPUserConn struct {
	*UserConn
	conn       *net.UDPConn
	userCh     chan []byte
	tunnelCh   chan []byte
	remoteAddr *net.UDPAddr
	timeout    sync.Map
}

func NewUDPUserConn(userConn *UserConn, conn *net.UDPConn, remoteAddr *net.UDPAddr) *UDPUserConn {
	return &UDPUserConn{
		UserConn:   userConn,
		userCh:     make(chan []byte, 10),
		tunnelCh:   make(chan []byte, 10),
		conn:       conn,
		remoteAddr: remoteAddr,
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
				Ctl:       message.NewDataConn,
				ServiceID: u.proxyServer.ctlMsg.GetServiceID(),
				SessionID: u.GetSessionID(),
				Data:      data,
			}, u.conn, u.tunnelConn.remoteAddr)
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
