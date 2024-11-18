package client

import (
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
)

type TCPTunnel struct {
	*Tunnel
}

func NewTCPTunnel(tunnel *Tunnel) *TCPTunnel {
	return &TCPTunnel{
		Tunnel: tunnel,
	}
}

func (t *TCPTunnel) NewTunnel() {
	t.newTunnelConnF = t.newTunnelConn
	t.newLocalConnF = t.newLocalConn
	t.tunnelToLocalF = t.tunnelToLocal
	t.localToTunnelF = t.localToTunnel
	t.process()
}

func (t *TCPTunnel) newTunnelConn() bool {
	if t.ctlMsg.GetSessionID() == "" {
		logrus.Errorf("[%s] sessionID is empty", t.ctlMsg.GetServiceID())
		return false
	}
	addr := net.JoinHostPort(t.Config.ServerHost, t.Config.ServerPort)
	tunnelConn, err := util.CreateDialTCP(addr)
	if err != nil {
		logrus.Errorf("[%s] tunnel conn connect %v", t.ctlMsg.GetServiceID(), err)
		return false
	}
	t.tunnelConn = tunnelConn
	msg := &message.ControlMessage{
		Ctl:       message.NewTunnel,
		Service:   t.ctlMsg.GetService(),
		ServiceID: t.ctlMsg.GetServiceID(),
		SessionID: t.ctlMsg.GetSessionID(),
		Token:     t.ctlMsg.GetToken(),
	}
	err = message.WriteTCP(msg, t.tunnelConn)
	if err != nil {
		logrus.Errorf("[%s] send ctl message %v", t.ctlMsg.GetServiceID(), err)
		return false
	}
	return true
}

func (t *TCPTunnel) newLocalConn() bool {
	var err error
	t.localConn, err = util.CreateDialTCP(t.ctlMsg.Service.LocalAddr)
	if err != nil {
		logrus.Errorf("[%s] local connect %v", t.ctlMsg.GetServiceID(), err)
		return false
	}
	return true
}

func (t *TCPTunnel) tunnelToLocal() {
	defer t.Close()
	err := message.Copy(t.localConn, t.tunnelConn, t.ResetTimeout)
	if err != nil {
		logrus.Tracef("[%s] tunnel to user %v", t.ctlMsg.GetServiceID(), err)
	}
}

func (t *TCPTunnel) localToTunnel() {
	defer t.Close()
	err := message.Copy(t.tunnelConn, t.localConn, t.ResetTimeout)
	if err != nil {
		logrus.Tracef("[%s] user to tunnel %v", t.ctlMsg.GetServiceID(), err)
	}
}
