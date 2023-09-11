package server

import (
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
	"sync"
)

type TunnelConn struct {
	conn       net.Conn
	remoteAddr *net.UDPAddr
	ctlMsg     *message.ControlMessage
	oneClose   sync.Once
}

func NewTunnelConn(conn net.Conn, ctlMsg *message.ControlMessage, remoteAddr *net.UDPAddr) *TunnelConn {
	logrus.Infof("[%s] new tunnel sessionID:=%s", ctlMsg.GetServiceID(), ctlMsg.GetSessionID())
	return &TunnelConn{
		conn:       conn,
		ctlMsg:     ctlMsg,
		remoteAddr: remoteAddr,
	}
}

func (t *TunnelConn) Close() {
	t.oneClose.Do(func() {
		if t.conn != nil {
			_ = t.conn.Close()
		}
		logrus.Debugf("[%s] close tunnel sessionID:=%s", t.ctlMsg.GetServiceID(), t.ctlMsg.GetSessionID())
	})
}

func (t *TunnelConn) GetSessionID() string {
	return t.ctlMsg.GetSessionID()
}

func (t *TunnelConn) ReSetTimeout(clientTimeout int) {
	_ = util.SetReadDeadline(t.conn)(clientTimeout)
}
