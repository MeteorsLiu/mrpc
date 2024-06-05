//go:build linux

package internal

import (
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
	"golang.org/x/sys/unix"
)

func fillBuffer() []byte {
	buf := make([]byte, 32768)
	for i := range buf {
		buf[i] = byte(i)
	}
	return buf
}

func writeTest(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 32768)
	time.Sleep(5 * time.Second)
	log.Println("start read")
	io.ReadFull(conn, buf[:32768-20])
	pos := 0
	for {
		n, err := conn.Read(buf[pos : pos+1])
		if err != nil {
			log.Println(buf[:pos])
			return
		}
		pos += n
		time.Sleep(time.Second)
		log.Println("recv: ", buf[:pos])
	}
}

func helloWorldTest(conn net.Conn) {
	toSend := []byte("Hello")

	for _, s := range toSend {
		conn.Write([]byte{s})
		time.Sleep(1 * time.Second)
	}
	conn.Write([]byte("World"))
	conn.Close()
}

func newListener(wg *sync.WaitGroup, handle func(net.Conn)) {
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
		go handle(c)
	}
}

func TestBaseRead(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go newListener(&wg, helloWorldTest)
	wg.Wait()

	cn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		t.Error(err)
		return
	}
	hello := false
	wg.Add(1)
	_, err = NewBaseConn(cn, func(c reactor.Conn, b []byte) {
		//log.Println("recv: ", string(b))
		if len(b) < 5 {
			c.SetNextReadSize(5 - len(b))
			return
		}
		if !hello && string(b) != "Hello" {
			t.Errorf("Read Misbehave: %s %d want: Hello", string(b), len(b))
			return
		}
		if !hello {
			hello = true
			return
		}
		if string(b) != "World" {
			t.Errorf("Read Misbehave:  %s %d  want: World", string(b), len(b))
			return
		}

	}, func(c reactor.Conn, b []byte, err error) {
		wg.Done()
	})
	if err != nil {
		t.Error(err)
		return
	}
	wg.Wait()
}

func TestBaseWrite(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go newListener(&wg, writeTest)
	wg.Wait()

	cn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		t.Error(err)
		return
	}
	base, err := NewBaseConn(cn, nil, nil)

	if err != nil {
		t.Error(err)
		return
	}
	unix.SetsockoptInt(base.FD(), unix.SOL_SOCKET, unix.SO_SNDBUF, 10)

	// wrong example, only for testing.
	// MUST NOT call Write() or Close() directly without wrapper.
	t.Log(base.Write(fillBuffer()))
	// EAGAIN
	t.Log(base.Write([]byte("HelloWorld")))
	time.Sleep(5 * time.Minute)
	base.Close()

}
