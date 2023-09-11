package benchmark

import (
	"bufio"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestTCPServer(t *testing.T) {
	listener, err := net.Listen("tcp", "0.0.0.0:10000")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listen on %v", listener.Addr())
	mx := sync.Mutex{}
	count := 0
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		go func() {
			defer func() {
				_ = conn.Close()
			}()
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			reader := bufio.NewReader(conn)
			for {
				_, err := reader.ReadString('\n')
				if err != nil {
					log.Printf("error: %v", err)
					return
				}
				_, err = conn.Write([]byte("server hello\n"))
				if err != nil {
					log.Printf("error: %v", err)
					return
				}
				mx.Lock()
				count++
				log.Printf("count: %d", count)
				mx.Unlock()
			}
		}()
	}
}

func TestTCPDial(t *testing.T) {
	wg := new(sync.WaitGroup)
	mx := sync.Mutex{}
	count := 0
	_i := os.Getenv("TEST_NUM")
	__i, err := strconv.Atoi(_i)
	if err != nil {
		__i = 1
	}
	for i := 0; i < __i; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", "192.168.6.50:6101")
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			defer func() {
				_ = conn.Close()
			}()
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			_, err = conn.Write([]byte("hello\n"))
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			buf := make([]byte, 1024)
			_, err = conn.Read(buf)
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			mx.Lock()
			count++
			log.Printf("count: %d", count)
			mx.Unlock()
		}()
	}
	wg.Wait()
}

func BenchmarkTCPDial(b *testing.B) {
	limit := make(chan struct{}, 1)
	for i := 0; i < b.N; i++ {
		limit <- struct{}{}
		func() {
			conn, err := net.Dial("tcp", "192.168.6.50:6101")
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			defer func() {
				_ = conn.Close()
				<-limit
			}()
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			_, err = conn.Write([]byte("hello\n"))
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			buf := make([]byte, 1024)
			_, err = conn.Read(buf)
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
		}()
	}
}
