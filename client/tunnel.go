package client

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
	"sync"
)

type Tunnel struct {
	*Client
	ctlMsg         *message.ControlMessage
	tunnelConn     net.Conn
	localConn      net.Conn
	cancel         context.CancelFunc
	onceClose      sync.Once
	newTunnelConnF func() bool
	newLocalConnF  func() bool
	tunnelToUserF  func(*sync.WaitGroup)
	localToTunnelF func(*sync.WaitGroup)
}

func NewTunnel(control *Client, ctlMsg *message.ControlMessage) *Tunnel {
	return &Tunnel{
		Client: control,
		ctlMsg: ctlMsg,
	}
}

func (t *Tunnel) ResetTimeout() {
	_ = util.SetReadDeadline(t.localConn)(t.Config.ConnTimeout)
	_ = util.SetReadDeadline(t.tunnelConn)(t.Config.ConnTimeout)
}

func (t *Tunnel) Close() {
	t.onceClose.Do(func() {
		if t.localConn != nil {
			_ = t.localConn.Close()
		}
		if t.tunnelConn != nil {
			if t.cancel != nil {
				t.cancel()
			}
			_ = t.tunnelConn.Close()
		}
	})
}

func (t *Tunnel) process() {
	defer t.Close()
	logrus.Infof("[%s] new request sessionID:=%s", t.ctlMsg.GetServiceID(), t.ctlMsg.GetSessionID())
	if !t.newTunnelConnF() {
		return
	}
	if !t.newLocalConnF() {
		return
	}
	t.ResetTimeout()
	wg := new(sync.WaitGroup)
	wg.Add(2)
	// 转发隧道数据到本地服务
	go t.tunnelToUserF(wg)
	// 转发本地服务数据到隧道
	go t.localToTunnelF(wg)
	wg.Wait()
	logrus.Debugf("[%s] close tunnel sessionID=%s", t.ctlMsg.GetServiceID(), t.ctlMsg.GetSessionID())
}
