package benchmark

import (
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestUDPServer(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "0.0.0.0:10000")
	conn, err := net.ListenUDP("udp", addr)
	count := 0
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listen on %v", conn.LocalAddr())
	defer conn.Close()
	for {
		buf := make([]byte, 1024)
		_, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		_, err = conn.WriteTo([]byte("server hello\n"), remoteAddr)
		if err != nil {
			log.Printf("error: %v", err)
		}
		count++
		log.Printf("count: %d", count)
	}
}

func TestUDPDial(t *testing.T) {
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
			conn, err := net.Dial("udp", "192.168.6.50:6101")
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			defer func() {
				_ = conn.Close()
			}()
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			_, err = conn.Write([]byte("client hello\n"))
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

func BenchmarkUDPDial(b *testing.B) {
	limit := make(chan struct{}, 1)
	for i := 0; i < b.N; i++ {
		limit <- struct{}{}
		go func() {
			conn, err := net.Dial("udp", "192.168.6.50:6101")
			if err != nil {
				log.Printf("error: %v", err)
				return
			}
			defer func() {
				_ = conn.Close()
				<-limit
			}()
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			_, err = conn.Write([]byte("client hello\n"))
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
