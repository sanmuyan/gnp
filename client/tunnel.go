package client

import (
	"github.com/sirupsen/logrus"
	"gnp/message"
	"gnp/util"
	"net"
	"sync"
)

type Tunnel interface {
	Process()
}

type TCPTunnel struct {
	*Control
	msg        *message.Message
	tunnelConn net.Conn
	localConn  net.Conn
}

func NewTCPTunnel(control *Control, msg *message.Message) Tunnel {
	return &TCPTunnel{Control: control, msg: msg}
}

func (t *TCPTunnel) newTunnel() bool {
	tunnelConn, err := util.CreateDialTCP(net.JoinHostPort(t.Control.Config.ServerHost, t.Control.Config.ServerPort))
	if err != nil {
		logrus.Errorln("tunnel connect", err)
		return false
	}
	localConn, err := util.CreateDialTCP(t.msg.Service.LocalAddr)
	if err != nil {
		logrus.Errorln("local connect", err)
		_ = t.tunnelConn.Close()
		return false
	}
	t.tunnelConn = tunnelConn
	t.localConn = localConn
	err = t.Control.SendCtl(t.tunnelConn, t.msg, message.NewTunnelCtl)
	if err != nil {
		logrus.Errorf("send ctl message %+v %v", t.msg, err)
		return false
	}
	return true
}

func (t *TCPTunnel) Process() {
	// 向服务端发起隧道连接
	if !t.newTunnel() {
		return
	}
	logrus.Infof("[%s] request %s %s", t.msg.Service.Network+t.msg.Service.ProxyPort, t.msg.Data, t.localConn.RemoteAddr())
	go util.ForwardConn(t.localConn, t.tunnelConn)
	go util.ForwardConn(t.tunnelConn, t.localConn)
}

type UDPTunnel struct {
	*Control
	tunnelConn   *net.UDPConn
	localConn    *net.UDPConn
	newTunnelMsg *message.Message
}

func NewUDPTunnel(control *Control, msg *message.Message) Tunnel {
	return &UDPTunnel{
		Control:      control,
		newTunnelMsg: msg,
	}
}

func (t *UDPTunnel) connSet() {
	_ = t.tunnelConn.SetWriteBuffer(message.UDPConnBufferSize)
	_ = t.localConn.SetWriteBuffer(message.UDPConnBufferSize)
	_ = util.SetReadDeadline(t.tunnelConn)(t.Config.UDPTunnelTimeOut)
	_ = util.SetReadDeadline(t.localConn)(t.Config.UDPTunnelTimeOut)
}

func (t *UDPTunnel) Close() {
	t.newTunnelMsg.Ctl = message.TunnelConnClose
	_, err := t.tunnelConn.Write(t.newTunnelMsg.EncodeUDP())
	if err != nil {
		logrus.Errorln("write tunnel message", err)
	}
	_ = t.tunnelConn.Close()
}

func (t *UDPTunnel) newTunnel() bool {
	tunnelConn, err := util.CreateDialUDP(net.JoinHostPort(t.Control.Config.ServerHost, t.newTunnelMsg.Service.ProxyPort))
	if err != nil {
		logrus.Errorln("tunnel connect", err)
		return false
	}
	localConn, err := util.CreateDialUDP(t.newTunnelMsg.Service.LocalAddr)
	if err != nil {
		logrus.Errorln("local connect", err)
		return false
	}
	t.tunnelConn = tunnelConn
	t.localConn = localConn
	_, err = t.tunnelConn.Write(t.newTunnelMsg.EncodeUDP())
	if err != nil {
		return false
	}
	t.connSet()
	return true
}

func (t *UDPTunnel) Process() {
	defer t.Close()

	// 向服务端发起隧道连接
	if !t.newTunnel() {
		return
	}
	logrus.Infof("[%s] request %s %s", t.newTunnelMsg.Service.Network+t.newTunnelMsg.Service.ProxyPort, t.newTunnelMsg.SessionID, t.localConn.RemoteAddr())

	wg := new(sync.WaitGroup)
	wg.Add(2)
	go t.tunnelToLocal(wg)
	go t.localToTunnel(wg)
	wg.Wait()
}

func (t *UDPTunnel) tunnelToLocal(wg *sync.WaitGroup) {
	// 读取隧道中的请求并转发到本地
	defer wg.Done()
	defer func() {
		_ = t.localConn.Close()
	}()
	message.ReadAndUnmarshalUDP(t.tunnelConn, func(msg *message.Message, err error) (exit bool) {
		if err != nil {
			logrus.Debugln("read tunnel data", err)
			return true
		}
		_ = util.SetReadDeadline(t.tunnelConn)(30)
		_, err = t.localConn.Write(msg.Data)
		if err != nil {
			logrus.Debugln("write local data", err)
		}
		return false
	})
}

func (t *UDPTunnel) localToTunnel(wg *sync.WaitGroup) {
	// 读取本地响应并写入隧道
	defer wg.Done()
	message.ReadDataUDP(t.localConn, func(msg *message.Message, err error) (exit bool) {
		if err != nil {
			logrus.Debugln("read local data", err)
			return true
		}
		_ = util.SetReadDeadline(t.localConn)(30)
		msg.Ctl = message.TunnelDataConnCtl
		msg.SessionID = t.newTunnelMsg.SessionID
		_, err = t.tunnelConn.Write(msg.EncodeUDP())
		if err != nil {
			logrus.Debugln("write tunnel data", err)
		}
		return false
	})
}
