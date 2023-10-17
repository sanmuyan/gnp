package server

import (
	"bufio"
	"context"
	"github.com/sanmuyan/xpkg/xnet"
	"github.com/sirupsen/logrus"
	"gnp/pkg/config"
	"gnp/pkg/message"
	"gnp/pkg/util"
	"io"
	"net"
	"sync"
)

type Server struct {
	Config         config.ServerConfig
	tunnelConnPool map[string]chan *TunnelConn
	mx             sync.Mutex
}

func NewServer(config config.ServerConfig) *Server {
	return &Server{
		Config:         config,
		tunnelConnPool: make(map[string]chan *TunnelConn),
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
		proxy := NewUDPProxy(NewProxyServer(ctx, s, conn, msg))
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

func (s *Server) controller(conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()
	reader := bufio.NewReaderSize(conn, message.ReadBufferSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := message.ReadTCP(reader)
			if err != nil {
				if err != io.EOF {
					logrus.Infof("server connect closed %v", err)
				} else {
					logrus.Errorf("read ctl message %v", err)
				}
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
			}

		}
	}
}

func (s *Server) handleConn(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logrus.Errorf("accept conn %v", err)
			continue
		}
		go s.controller(conn)
	}
}

func Run() {
	listener, err := util.CreateListenTCP(config.ServerConf.ServerBind, config.ServerConf.ServerPort)
	if err != nil {
		logrus.Fatalf("server listen %v", err)
	}
	logrus.Infof("server running %s", net.JoinHostPort(config.ServerConf.ServerBind, config.ServerConf.ServerPort))
	s := NewServer(config.ServerConf)
	s.handleConn(listener)
}
