package util

import (
	"net"
	"time"
)

func CreateListenTCP(addr, port string) (net.Listener, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(addr, port))
	if err != nil {
		return listener, err
	}
	return listener, nil
}

func CreateDialTCP(addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return conn, err
	}
	return conn, err
}

func CreateListenUDP(addr, port string) (*net.UDPConn, error) {
	updAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(addr, port))
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", updAddr)
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

func CreateDialUDP(addr string) (*net.UDPConn, error) {
	updAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.DialUDP("udp", nil, updAddr)
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

func SetReadDeadline(conn net.Conn) func(int) error {
	return func(s int) error {
		if s == 0 {
			s = 3600
		}
		err := conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s)))
		if err != nil {
			return err
		}
		return nil
	}
}
