package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

// UserConnProvider 用户连接处理提供者
type UserConnProvider interface {
	// IsTunnelAvailable 隧道连接是否可用
	IsTunnelAvailable() bool
	// SetTunnelConn 设置隧道连接
	SetTunnelConn(*TunnelConn)
	// GetCreateTime 获取用户连接创建时间
	GetCreateTime() int64
	// GetSessionID 获取用户连接的会话 ID
	GetSessionID() string
	// Close 关闭用户连接
	Close()
	// UserToTunnel 用户数据转发到隧道
	UserToTunnel()
	// TunnelToUser 隧道数据转发到用户
	TunnelToUser()
}

// UserConn 处理用户连接
type UserConn struct {
	ctx         context.Context
	cancel      context.CancelFunc
	proxyServer *ProxyServer
	// createTime 用户连接创建时间
	createTime int64
	// isTunnelAvailable 隧道连接是否可用
	isTunnelAvailable bool
	// sessionID 用户连接的会话 ID
	sessionID string
	// oneClose 避免重复关闭用户连接引发异常
	oneClose sync.Once
	// tunnelConn 隧道连接信息
	tunnelConn *TunnelConn
}

func NewUserConn(ctx context.Context, cancel context.CancelFunc, proxyServer *ProxyServer, sessionID string) *UserConn {
	return &UserConn{
		ctx:         ctx,
		cancel:      cancel,
		createTime:  time.Now().Unix(),
		proxyServer: proxyServer,
		sessionID:   sessionID,
	}
}

func (u *UserConn) GetCreateTime() int64 {
	return u.createTime
}

func (u *UserConn) GetSessionID() string {
	return u.sessionID
}

func (u *UserConn) SetTunnelAvailable(x bool) {
	u.isTunnelAvailable = x
}

func (u *UserConn) IsTunnelAvailable() bool {
	return u.isTunnelAvailable
}

func (u *UserConn) Close() {
	u.oneClose.Do(func() {
		u.cancel()
		if u.isTunnelAvailable {
			u.tunnelConn.Close()
		}
		u.proxyServer.RemoveUserConn(u.GetSessionID())
		logrus.Debugf("[%s] close userConn sessionID:=%s", u.proxyServer.ctlMsg.GetServiceID(), u.GetSessionID())
	})
}

func (u *UserConn) SetTunnelConn(tunnelConn *TunnelConn) {
	u.tunnelConn = tunnelConn
	u.SetTunnelAvailable(true)
}
