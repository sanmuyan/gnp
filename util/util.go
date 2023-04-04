package util

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net"
	"strconv"
	"strings"
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

func CreatePassword(password string) (hashPassword string) {
	p := []byte(password)
	h, _ := bcrypt.GenerateFromPassword(p, bcrypt.MinCost)
	hashPassword = string(h)
	return hashPassword
}

func ComparePassword(hashPassword string, password string) bool {
	p := []byte(password)
	h := []byte(hashPassword)
	err := bcrypt.CompareHashAndPassword(h, p)
	if err != nil {
		return false
	}
	return true
}

func IsAllowPort(allowPorts, bindPort string) bool {
	bindPortInt, err := strconv.Atoi(bindPort)
	if err != nil {
		return false
	}

	allowPortsFields := strings.FieldsFunc(allowPorts, func(r rune) bool {
		return r == ','
	})

	var allowPortsArray [][]string

	for _, port := range allowPortsFields {
		if strings.Contains(port, "-") {
			portRange := strings.FieldsFunc(port, func(r rune) bool {
				return r == '-'
			})
			if len(portRange) == 2 {
				allowPortsArray = append(allowPortsArray, portRange)
			}
		} else {
			allowPortsArray = append(allowPortsArray, []string{port})
		}
	}

	for _, port := range allowPortsArray {
		if len(port) == 1 {
			portInt, err := strconv.Atoi(port[0])
			if err != nil {
				continue
			}
			if portInt == bindPortInt && portInt >= 0 && portInt <= 65535 {
				return true
			}
		}
		if len(port) == 2 {
			minPort, err := strconv.Atoi(port[0])
			if err != nil {
				continue
			}
			maxPort, err := strconv.Atoi(port[1])
			if err != nil {
				continue
			}
			if bindPortInt >= minPort && bindPortInt <= maxPort && maxPort >= minPort && minPort >= 0 && maxPort <= 65535 {
				return true
			}
		}
	}
	return false
}

func ForwardConn(src, dst net.Conn) {
	defer func() {
		_ = src.Close()
	}()
	_, err := io.Copy(src, dst)
	if err != nil {
		logrus.Traceln("io_forward", err)
	}
}

func Encode(data []byte) ([]byte, error) {
	length := int32(len(data))
	pkg := new(bytes.Buffer)
	err := binary.Write(pkg, binary.LittleEndian, length)
	if err != nil {
		return nil, err
	}
	err = binary.Write(pkg, binary.LittleEndian, data)
	if err != nil {
		return nil, err
	}
	return pkg.Bytes(), nil
}

var BufOverflow = errors.New("BufOverflow")

func Decode(reader *bufio.Reader) ([]byte, error) {
	lengthByte, err := reader.Peek(4)
	if err != nil {
		return nil, err
	}
	lengthBuf := bytes.NewBuffer(lengthByte)
	var length int32
	err = binary.Read(lengthBuf, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}
	if int32(reader.Buffered()) < length+4 {
		return nil, BufOverflow
	}

	pkg := make([]byte, int(4+length))
	_, err = reader.Read(pkg)
	if err != nil {
		return nil, err
	}
	return pkg[4:], nil
}

func SetReadDeadline(conn net.Conn) func(int) error {
	return func(s int) error {
		if s == 0 {
			s = 60
		}
		err := conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s)))
		if err != nil {
			return err
		}
		return nil
	}
}
