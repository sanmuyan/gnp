package client

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"net"
	"sync"
)

// Tunnel 隧道数据转发
type Tunnel struct {
	*Client
	ctx    context.Context
	cancel context.CancelFunc
	// ctlMsg 隧道连接控制消息
	ctlMsg *message.ControlMessage
	// tunnelConn 隧道连接
	tunnelConn net.Conn
	// localConn 本地服务连接
	localConn net.Conn
	// onceClose 避免重复关闭引发异常
	onceClose sync.Once
	// tunnelToLocalF 新建隧道连接
	newTunnelConnF func() bool
	// localToTunnelF 新建本地连接
	newLocalConnF func() bool
	// tunnelToLocalF 转发隧道数据到本地服务
	tunnelToLocalF func()
	// localToTunnelF 转发本地服务数据到隧道
	localToTunnelF func()
}

func NewTunnel(ctx context.Context, control *Client, ctlMsg *message.ControlMessage) *Tunnel {
	ctx, cancel := context.WithCancel(ctx)
	return &Tunnel{
		ctx:    ctx,
		cancel: cancel,
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
		t.cancel()
		if t.localConn != nil {
			_ = t.localConn.Close()
		}
		if t.tunnelConn != nil {
			_ = t.tunnelConn.Close()
		}
	})
}

func (t *Tunnel) process() {
	defer t.Close()
	if !t.newTunnelConnF() {
		return
	}
	if !t.newLocalConnF() {
		return
	}
	t.ResetTimeout()
	// 转发隧道数据到本地服务
	go t.tunnelToLocalF()
	// 转发本地服务数据到隧道
	go t.localToTunnelF()
	<-t.ctx.Done()
	logrus.Debugf("[%s] close tunnel sessionID=%s", t.ctlMsg.GetServiceID(), t.ctlMsg.GetSessionID())
}
