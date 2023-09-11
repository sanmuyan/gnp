package client

import (
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
	"sync"
)

type UDPTunnel struct {
	*Tunnel
}

func NewUDPTunnel(tunnel *Tunnel) *UDPTunnel {
	return &UDPTunnel{
		Tunnel: tunnel,
	}
}

func (t *UDPTunnel) NewTunnel() {
	t.newTunnelConnF = t.newTunnelConn
	t.newLocalConnF = t.newLocalConn
	t.tunnelToUserF = t.tunnelToLocal
	t.localToTunnelF = t.localToTunnel
	t.process()
}

func (t *UDPTunnel) newTunnelConn() bool {
	if t.ctlMsg.GetSessionID() == "" {
		logrus.Errorf("[%s] sessionID is empty", t.ctlMsg.GetServiceID())
		return false
	}
	tunnelConn, err := util.CreateDialUDP(net.JoinHostPort(t.Config.ServerHost, t.ctlMsg.GetService().GetProxyPort()))
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
	err = message.WriteUDP(msg, t.tunnelConn)
	if err != nil {
		logrus.Errorf("[%s] send ctl message %v", t.ctlMsg.GetServiceID(), err)
		return false
	}
	return true
}

func (t *UDPTunnel) newLocalConn() bool {
	var err error
	t.localConn, err = util.CreateDialUDP(t.ctlMsg.Service.LocalAddr)
	if err != nil {
		logrus.Errorf("[%s] local connect %v", t.ctlMsg.GetServiceID(), err)
		return false
	}
	return true
}

func (t *UDPTunnel) tunnelToLocal(wg *sync.WaitGroup) {
	defer wg.Done()
	defer t.Close()
	for {
		msg, err := message.ReadUDP(t.tunnelConn)
		if err != nil {
			logrus.Tracef("[%s] tunnel to user %v", t.ctlMsg.GetServiceID(), err)
			return
		}
		if msg.GetCtl() != message.NewDataConn || msg.GetServiceID() != t.ctlMsg.GetServiceID() || msg.GetSessionID() != t.ctlMsg.GetSessionID() {
			logrus.Warnf("[%s] tunnel data invalid", t.ctlMsg.GetServiceID())
			continue
		}
		_, err = t.localConn.Write(msg.Data)
		if err != nil {
			logrus.Tracef("[%s] tunnel to user %v", t.ctlMsg.GetServiceID(), err)
			return
		}
		t.ResetTimeout()
	}
}

func (t *UDPTunnel) localToTunnel(wg *sync.WaitGroup) {
	defer wg.Done()
	defer t.Close()
	for {
		buf := make([]byte, message.BufDataSize)
		n, err := t.localConn.Read(buf)
		if err != nil {
			logrus.Tracef("[%s] user to tunnel %v", t.ctlMsg.GetServiceID(), err)
			return
		}
		err = message.WriteUDP(&message.ControlMessage{
			Ctl:       message.NewDataConn,
			ServiceID: t.ctlMsg.GetServiceID(),
			SessionID: t.ctlMsg.GetSessionID(),
			Data:      buf[:n],
		}, t.tunnelConn)
		if err != nil {
			logrus.Tracef("[%s] user to tunnel %v", t.ctlMsg.GetServiceID(), err)
			return
		}
		t.ResetTimeout()
	}
}
