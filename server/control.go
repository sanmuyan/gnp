package server

import (
	"bufio"
	"context"
	"errors"
	"github.com/sanmuyan/xpkg/xnet"
	"github.com/sirupsen/logrus"
	"gnp/pkg/config"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"io"
	"net"
	"sync"
)

// Server 控制中心
type Server struct {
	Config config.ServerConfig
	// tunnelConnPool 新建隧道消息池，存储通知代理服务隧道连接信息的队列
	tunnelConnPool map[string]chan *TunnelConn
	// tunnelDataPool UDP 隧道数据池，存储代理服务的接收隧道数据的队列
	tunnelDataPool map[string]chan *TunnelData
	// udpTunnelConn UDP 控制连接，接收 UDP 隧道控制消息，接收和发送隧道数据
	udpTunnelConn *net.UDPConn
	mx            sync.Mutex
	wg            *sync.WaitGroup
}

func NewServer(config config.ServerConfig) *Server {
	return &Server{
		Config:         config,
		tunnelConnPool: make(map[string]chan *TunnelConn),
		tunnelDataPool: make(map[string]chan *TunnelData),
	}
}

func (s *Server) SendMsg(conn net.Conn, msg *message.ControlMessage) error {
	msg.Token = s.Config.Token
	return message.WriteTCP(msg, conn)
}

func (s *Server) auth(msg *message.ControlMessage) bool {
	if s.Config.Token == msg.GetToken() {
		return true
	}
	return false
}

func (s *Server) Clean(serviceID string) {
	s.mx.Lock()
	defer s.mx.Unlock()
	delete(s.tunnelConnPool, serviceID)
	delete(s.tunnelDataPool, serviceID)
}

func (s *Server) handelService(ctx context.Context, msg *message.ControlMessage, conn net.Conn) {
	if msg.GetServiceID() == "" {
		logrus.Warnf("registry service serviceID is empty client=%s", conn.RemoteAddr().String())
		return
	}
	logrus.Infof("[%s] registry service client=%s", msg.GetServiceID(), conn.RemoteAddr().String())
	if !xnet.IsAllowPort(s.Config.AllowPorts, msg.GetService().GetProxyPort()) {
		logrus.Warnf("[%s] not allowed port", msg.GetServiceID())
		return
	}
	if _, ok := s.tunnelConnPool[msg.GetServiceID()]; ok {
		logrus.Warnf("[%s] service is already registered", msg.GetServiceID())
		return
	}
	switch msg.GetService().GetNetwork() {
	case "tcp":
		proxy := NewTCPProxy(NewProxyServer(ctx, s, conn, msg))
		s.tunnelConnPool[msg.GetServiceID()] = proxy.tunnelConnCh
		go proxy.Start()
	case "udp":
		proxy := NewUDPProxy(NewProxyServer(ctx, s, conn, msg), s.udpTunnelConn)
		s.tunnelConnPool[msg.GetServiceID()] = proxy.tunnelConnCh
		s.tunnelDataPool[msg.GetServiceID()] = proxy.tunnelData
		go proxy.Start()
	}
	readyMsg := &message.ControlMessage{
		Ctl:       message.ServiceReady,
		Service:   msg.GetService(),
		ServiceID: msg.GetServiceID(),
		SessionID: msg.GetSessionID(),
	}
	err := s.SendMsg(conn, readyMsg)
	if err != nil {
		logrus.Errorf("[%s] send ctl message %v", msg.ServiceID, err)
		return
	}
}

// controller 处理服务端控制消息
func (s *Server) controller(ctx context.Context, conn net.Conn) {
	var isNewTunnelConn bool
	s.wg.Add(1)
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		// 如果是隧道连接不能关闭，否则隧道会断开
		if !isNewTunnelConn {
			_ = conn.Close()
		}
		cancel()
		s.wg.Done()
	}()
	reader := bufio.NewReaderSize(conn, message.ReadBufferSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := message.ReadTCP(reader)
			if err != nil {
				if err == io.EOF || errors.Is(err, net.ErrClosed) {
					logrus.Infof("client connect closed %s", conn.RemoteAddr().String())
					return
				}
				logrus.Errorf("read ctl message %v", err)
				return
			}
			if !s.auth(msg) {
				logrus.Warnf("auth failed client=%s", conn.RemoteAddr().String())
				continue
			}
			switch msg.GetCtl() {
			case message.NewTunnel:
				// 隧道连接加入对应代理队列
				if _, ok := s.tunnelConnPool[msg.GetServiceID()]; !ok {
					logrus.Warnf("[%s] service is not registered", msg.GetServiceID())
					return
				}
				s.tunnelConnPool[msg.GetServiceID()] <- NewTunnelConn(conn, msg, nil)
				isNewTunnelConn = true
				// 隧道连接需要直接 return 退出循环，否则代理转发逻辑无法读取隧道连接
				return
			case message.NewService:
				// 处理客户端服务代理注册
				s.handelService(ctx, msg, conn)
				continue
			case message.KeepAlive:
				err := s.SendMsg(conn, &message.ControlMessage{
					Ctl: message.KeepAlive,
				})
				if err != nil {
					logrus.Errorf("send keep alive message %v", err)
				}
				continue
			default:
				logrus.Warnf("[%s] unknown ctl:=%d", msg.GetServiceID(), msg.GetCtl())
			}

		}
	}
}

// udpController 处理 UDP 控制消息和数据
func (s *Server) udpController(data []byte, remoteAddr *net.UDPAddr) {
	msg, err := message.Unmarshal(data)
	if err != nil {
		logrus.Warnf("unmarshal udp tunnel %s", err)
		return
	}
	switch msg.GetCtl() {
	case message.NewTunnel:
		s.tunnelConnPool[msg.GetServiceID()] <- NewTunnelConn(nil, msg, remoteAddr)
	case message.NewTunnelData:
		s.tunnelDataPool[msg.GetServiceID()] <- NewTunnelData(msg, remoteAddr)
	default:
		logrus.Warnf("[%s] unknown ctl:=%d", msg.GetServiceID(), msg.GetCtl())
	}
}

func (s *Server) handleConn(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.Errorf("accept conn %v", err)
			return
		}
		go s.controller(ctx, conn)
	}
}

func (s *Server) handleUDPConn() {
	for {
		buf := make([]byte, message.BufDataSize)
		n, remoteAddr, err := s.udpTunnelConn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.Errorf("read udp tunnel %s", err)
			return
		}
		go s.udpController(buf[:n], remoteAddr)
	}
}

func Run(ctx context.Context) {
	listener, err := util.CreateListenTCP(config.ServerConf.ServerBind, config.ServerConf.ServerPort)
	if err != nil {
		logrus.Fatalf("server listen %v", err)
	}
	logrus.Infof("server listening on %s", net.JoinHostPort(config.ServerConf.ServerBind, config.ServerConf.ServerPort))
	s := NewServer(config.ServerConf)
	udpTunnelConn, err := util.CreateListenUDP(config.ServerConf.ServerBind, config.ServerConf.ServerPort)
	if err != nil {
		logrus.Fatalf("server listen %v", err)
	}
	s.udpTunnelConn = udpTunnelConn
	defer func() {
		_ = listener.Close()
		_ = s.udpTunnelConn.Close()
	}()
	s.wg = new(sync.WaitGroup)
	go s.handleUDPConn()
	go s.handleConn(ctx, listener)
	<-ctx.Done()
	s.wg.Wait()
}
