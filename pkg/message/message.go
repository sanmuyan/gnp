package message

import (
	"bufio"
	"errors"
	"github.com/sanmuyan/xpkg/xnet"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
)

//go:generate protoc --go_out=../ *.proto

const (
	NewTunnel = iota + 10000
	NewService
	ServiceReady
	KeepAlive
	NewDataConn
)

const (
	MTU             = 1500
	ReadBufferSize  = 4096 * 8
	WriteBufferSize = 1024 * 8
	BufDataSize     = MTU * 2
)

func Unmarshal(data []byte) (*ControlMessage, error) {
	msg := new(ControlMessage)
	return msg, proto.Unmarshal(data, msg)
}

func Marshal(msg *ControlMessage) ([]byte, error) {
	return proto.Marshal(msg)
}

func WriteTCP(msg *ControlMessage, conn net.Conn) error {
	bp, err := Marshal(msg)
	if err != nil {
		return err
	}
	be, err := xnet.Encode(bp)
	if err != nil {
		return err
	}
	_, err = conn.Write(be)
	return err
}

func ReadTCP(reader *bufio.Reader) (*ControlMessage, error) {
	be, err := xnet.Decode(reader)
	if err != nil {
		return nil, err
	}
	return Unmarshal(be)
}

func WriteUDP(msg *ControlMessage, conn net.Conn) error {
	bp, err := Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.Write(bp)
	return err
}

func WriteToUDP(msg *ControlMessage, conn *net.UDPConn, remoteAddr *net.UDPAddr) error {
	bp, err := Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(bp, remoteAddr)
	return err
}

func ReadUDP(conn net.Conn) (*ControlMessage, error) {
	buf := make([]byte, BufDataSize)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return Unmarshal(buf[:n])
}

func Copy(dst, src net.Conn, resetTimeout func()) error {
	buf := make([]byte, BufDataSize)
	var err error
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write")
				}
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
		resetTimeout()
	}
	return err
}
