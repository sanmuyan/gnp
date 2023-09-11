package server

import (
	"context"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type UserConnProvider interface {
	IsTunnelAvailable() bool
	SetTunnelConn(*TunnelConn)
	GetCreateTime() int64
	GetSessionID() string
	Close()
	UserToTunnel()
	TunnelToUser()
}

type UserConn struct {
	ctx               context.Context
	cancel            context.CancelFunc
	createTime        int64
	isTunnelAvailable bool
	sessionID         string
	oneClose          sync.Once
	proxyServer       *ProxyServer
	tunnelConn        *TunnelConn
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
