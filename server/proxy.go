package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/message"
	"gnp/util"
	"net"
	"sync"
	"time"
)

type Proxy interface {
	Start()
	Close()
}

type TCPProxy struct {
	listener net.Listener
	*Control
	ctlConn       net.Conn
	userConnCh    chan net.Conn
	tunnelConnCh  chan net.Conn
	newServiceMsg *message.Message
	ctx           context.Context
}

func NewTCPProxy(ctx context.Context, ctl *Control, newServiceMsg *message.Message, ctlConn net.Conn) *TCPProxy {
	return &TCPProxy{
		ctx:           ctx,
		Control:       ctl,
		newServiceMsg: newServiceMsg,
		ctlConn:       ctlConn,
		userConnCh:    make(chan net.Conn),
		tunnelConnCh:  make(chan net.Conn),
	}
}

func (p *TCPProxy) Start() {
	var err error
	p.listener, err = util.CreateListenTCP(p.Config.ServerBind, p.newServiceMsg.Service.ProxyPort)
	if err != nil {
		logrus.Errorln("proxy_listen", err)
		return
	}
	defer p.Close()
	go p.handelConn()
	go p.watchTunnel()
	<-p.ctx.Done()
}

func (p *TCPProxy) Close() {
	_ = p.listener.Close()
	logrus.Infoln("close service", p.newServiceMsg.Service.Network, p.newServiceMsg.Service.ProxyPort)
}

func (p *TCPProxy) handelConn() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			logrus.Debugln("accept proxy connect", err)
			return
		}
		go p.handelUserConn(conn)
	}
}

func (p *TCPProxy) watchTunnel() {
	// 有新隧道连接 转发用户和隧道连接
	for {
		select {
		case <-p.ctx.Done():
			return
		case t := <-p.tunnelConnCh:
			u := <-p.userConnCh
			go util.ForwardConn(t, u)
			go util.ForwardConn(u, t)
		}
	}
}

func (p *TCPProxy) handelUserConn(conn net.Conn) {
	// 把用户连接加入队列并通知客户端建立隧道连接
	logrus.Infof("[%s] request %s", p.newServiceMsg.Service.Network+p.newServiceMsg.Service.ProxyPort, conn.RemoteAddr())
	err := p.Control.SendCtl(p.ctlConn, message.NewMessage(&message.Options{
		Service:   p.newServiceMsg.Service,
		SessionID: p.newServiceMsg.SessionID,
		Data:      []byte(conn.RemoteAddr().String()),
	}), message.NewTunnelCtl)
	if err != nil {
		logrus.Errorf("send ctl message %+v %v", p.newServiceMsg, err)
	}
	p.userConnCh <- conn
}

type UDPProxy struct {
	*Control
	conn                  *net.UDPConn
	ctlConn               net.Conn
	tunnelAddrWaiting     sync.Map
	tunnelAddrCh          sync.Map
	tunnelAddrPool        sync.Map
	tunnelAddrClean       sync.Map
	tunnelAddrCleanTicker *time.Ticker
	newServiceMsg         *message.Message
	ctx                   context.Context
}

func NewUDPProxy(ctx context.Context, ctl *Control, newServiceMsg *message.Message, ctlConn net.Conn) *UDPProxy {
	return &UDPProxy{
		ctx:                   ctx,
		Control:               ctl,
		newServiceMsg:         newServiceMsg,
		ctlConn:               ctlConn,
		tunnelAddrCleanTicker: time.NewTicker(time.Second * 5),
	}
}

func (p *UDPProxy) Start() {
	var err error
	p.conn, err = util.CreateListenUDP(p.Config.ServerBind, p.newServiceMsg.Service.ProxyPort)
	if err != nil {
		logrus.Errorln("proxy_listen", err)
		return
	}

	defer p.Close()
	p.connSet()
	go p.handleConn()
	go p.clean()
	<-p.ctx.Done()
}

func (p *UDPProxy) Close() {
	_ = p.conn.Close()
	p.tunnelAddrCleanTicker.Stop()
	logrus.Infoln("close service", p.newServiceMsg.Service.Network, p.newServiceMsg.Service.ProxyPort)
}

func (p *UDPProxy) handleConn() {
	// 读取并尝试序列化消息
	message.ReadOrUnmarshalUDP(p.conn, func(msg *message.Message, addr *net.UDPAddr, err error, errType int) (exit bool) {
		switch errType {
		case 1:
			logrus.Debugln("read proxy data", err)
			return true
		case 2:
			// 序列化失败则是用户数据
			go p.handelUserConn(msg, addr)
		default:
			// 序列化成功则是隧道数据
			go p.handelTunnelConn(msg, addr)
		}
		return false
	})
}

func (p *UDPProxy) handelUserConn(msg *message.Message, remoteAddr *net.UDPAddr) {
	// 通知客户端建立隧道连接 并把用户请求数据写入隧道
	sessionID := remoteAddr.String()
	tunnelAddr, ok := p.tunnelAddrPool.Load(sessionID)
	if !ok {
		p.tunnelAddrWaiting.Store(sessionID, true)
		logrus.Infof("[%s] request %s", p.newServiceMsg.Service.Network+p.newServiceMsg.Service.ProxyPort, remoteAddr.String())
		err := p.Control.SendCtl(p.ctlConn, message.NewMessage(&message.Options{
			Service:   p.newServiceMsg.Service,
			SessionID: sessionID,
		}), message.NewTunnelCtl)
		if err != nil {
			logrus.Errorf("send ctl message %+v %v", p.newServiceMsg, err)
			return
		}
		tunnelCh := make(chan *net.UDPAddr)
		p.tunnelAddrCh.Store(sessionID, tunnelCh)
		select {
		case <-time.After(time.Second):
			logrus.Errorln("wait tunnel timeout", remoteAddr)
			p.tunnelAddrWaiting.Delete(sessionID)
			return
		case tunnelAddr = <-tunnelCh:
			p.tunnelAddrPool.Store(sessionID, tunnelAddr)
			p.tunnelAddrWaiting.Delete(sessionID)
			defer p.tunnelAddrClean.Store(sessionID, time.Now().Unix())
		}
	}

	// 等待隧道连接时忽略重试请求
	if _, ok := p.tunnelAddrWaiting.Load(sessionID); !ok {
		_, err := p.conn.WriteTo(msg.EncodeUDP(), tunnelAddr.(*net.UDPAddr))
		if err != nil {
			logrus.Errorln("write to tunnel data", err)
		}
	} else {
		logrus.Warnln("ignore request waiting tunnel", sessionID)
	}
}

func (p *UDPProxy) handelTunnelConn(msg *message.Message, remoteAddr *net.UDPAddr) {
	switch msg.Ctl {
	case message.NewTunnelCtl:
		// 收到隧道连接地址
		id, ok := p.tunnelAddrCh.Load(msg.SessionID)
		if ok {
			id.(chan *net.UDPAddr) <- remoteAddr
		}
	case message.TunnelDataConnCtl:
		// 隧道数据返回给用户
		addr, _ := net.ResolveUDPAddr("udp", msg.SessionID)
		_, err := p.conn.WriteTo(msg.Data, addr)
		if err != nil {
			logrus.Errorln("write to user data", err)
		}
		defer p.tunnelAddrClean.Store(msg.SessionID, time.Now().Unix())
	case message.TunnelConnClose:
		_, ok := p.tunnelAddrClean.Load(msg.SessionID)
		if ok {
			logrus.Debugln("tunnel close", msg.SessionID)
			defer p.removeSession(msg.SessionID)
		}
	}
}

func (p *UDPProxy) clean() {
	for range p.tunnelAddrCleanTicker.C {
		p.tunnelAddrClean.Range(func(id, latestTime any) bool {
			if time.Now().Unix()-latestTime.(int64) > int64(p.Config.UDPTunnelTimeOut) {
				logrus.Debugln("clean udp addr", id)
				p.removeSession(id)
			}
			return true
		})
	}
}

func (p *UDPProxy) connSet() {
	_ = p.conn.SetWriteBuffer(message.UDPConnBufferSize)
}

func (p *UDPProxy) removeSession(id any) {
	p.tunnelAddrPool.Delete(id)
	p.tunnelAddrCh.Delete(id)
	p.tunnelAddrClean.Delete(id)
}
