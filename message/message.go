package message

import (
	"bufio"
	"github.com/sirupsen/logrus"
	"gnp/util"
	"google.golang.org/protobuf/proto"
	"net"
	"sync"
)

//go:generate protoc --go_out=../ *.proto

const (
	KeepAliveCtl      = 1
	NewTunnelCtl      = 2
	NewServiceCtl     = 3
	ServiceReadyCtl   = 4
	TunnelDataConnCtl = 5
	TunnelConnClose   = 6
	LoginCtl          = 7
	BufferSize        = 4096 * 8
	MTU               = 1500
	UDPDataSize       = MTU * 2
	UDPConnBufferSize = 1024 * 8
)

type Message struct {
	*Options
}

type Call func(msg *Message, err error) (exit bool)

var Pool = &sync.Pool{
	New: func() any {
		return new(Message)
	},
}

func NewMessage(o *Options) *Message {
	m := Pool.Get().(*Message)
	m = &Message{
		Options: o,
	}
	return m
}

func (m *Message) EncodeTCP() []byte {
	ba, _ := proto.Marshal(m)
	bs, _ := util.Encode(ba)
	return bs
}

func (m *Message) EncodeUDP() []byte {
	ba, _ := proto.Marshal(m)
	return ba
}

func ReadMessageTCP(conn net.Conn, call Call) {
	reader := bufio.NewReaderSize(conn, BufferSize)
	for {
		bs, err := util.Decode(reader)
		if err != nil {
			if err == util.BufOverflow {
				reader.Reset(bufio.NewReaderSize(conn, BufferSize))
				logrus.Warnln("buf overflow reset reader")
				continue
			} else {
				call(nil, err)
				return
			}
		}

		m := Pool.Get().(*Message)
		m.Options = &Options{}
		err = proto.Unmarshal(bs, m)
		if err != nil {
			call(nil, err)
			return
		}
		if call(m, err) {
			return
		}
	}
}

func ReadAndUnmarshalUDP(conn *net.UDPConn, call Call) {
	reader := bufio.NewReaderSize(conn, BufferSize)
	for {
		buf := make([]byte, UDPDataSize)
		n, err := reader.Read(buf)
		if err != nil {
			call(nil, err)
			return
		}

		m := Pool.Get().(*Message)
		m.Options = &Options{}
		err = proto.Unmarshal(buf[:n], m)
		if err != nil {
			call(nil, err)
			return
		}
		if call(m, nil) {
			return
		}
	}
}

func ReadDataUDP(conn *net.UDPConn, call Call) {
	reader := bufio.NewReaderSize(conn, BufferSize)
	for {
		buf := make([]byte, UDPDataSize)
		n, err := reader.Read(buf)
		if err != nil {
			call(nil, err)
			return
		}

		m := Pool.Get().(*Message)
		m.Options = &Options{
			Data: buf[:n],
		}
		if call(m, nil) {
			return
		}
	}
}

func ReadOrUnmarshalUDP(conn *net.UDPConn, call func(msg *Message, addr *net.UDPAddr, err error, errType int) (exit bool)) {
	for {
		buf := make([]byte, UDPDataSize)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			call(nil, nil, err, 1)
			return
		}

		m := Pool.Get().(*Message)
		m.Options = &Options{}
		err = proto.Unmarshal(buf[:n], m)
		if err != nil {
			m.Data = buf[:n]
			call(m, addr, err, 2)
			continue
		}
		if call(m, addr, nil, 0) {
			return
		}
	}
}
