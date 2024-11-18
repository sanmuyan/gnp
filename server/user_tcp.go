package server

import (
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
)

// TCPUserConn 转发用户 TCP 连接
type TCPUserConn struct {
	*UserConn
	// conn 用户的 TCP 连接
	conn net.Conn
}

func NewTCPUserConn(userConn *UserConn, conn net.Conn) *TCPUserConn {
	return &TCPUserConn{UserConn: userConn, conn: conn}
}

func (u *TCPUserConn) ResetTimeout() {
	_ = util.SetReadDeadline(u.conn)
}

func (u *TCPUserConn) UserToTunnel() {
	defer u.Close()
	err := message.Copy(u.tunnelConn.conn, u.conn, u.ResetTimeout)
	if err != nil {
		logrus.Tracef("[%s] user to tunnel %v", u.proxyServer.ctlMsg.GetServiceID(), err)
	}
}

func (u *TCPUserConn) TunnelToUser() {
	defer u.Close()
	err := message.Copy(u.conn, u.tunnelConn.conn, u.ResetTimeout)
	if err != nil {
		logrus.Tracef("[%s] tunnel to user %v", u.proxyServer.ctlMsg.GetServiceID(), err)
	}
}
