//go:build linux

package internal

import (
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
)

func newListener(wg *sync.WaitGroup) {
	l, err := net.Listen("tcp", "127.0.0.1:9999")
	if err != nil {
		log.Fatal(err)
	}
	wg.Done()

	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			toSend := []byte("Hello")

			for _, s := range toSend {
				conn.Write([]byte{s})
				time.Sleep(1 * time.Second)
			}
			conn.Close()
		}(c)
	}
}

func onRead(c reactor.Conn, b []byte, err error) {
	log.Println("recv: ", string(b))
	if len(b) < 5 {
		c.SetNextReadSize(5 - len(b))
		return
	}

}

func TestBase(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go newListener(&wg)
	wg.Wait()

	cn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		t.Error(err)
		return
	}
	wg.Add(1)
	_, err = NewBaseConn(cn, onRead, func(c reactor.Conn, b []byte, err error) {
		wg.Done()
	})
	if err != nil {
		t.Error(err)
		return
	}
	wg.Wait()
}
