package reactor

import "net"

type Conn interface {
	net.Conn
	FD() int
	SetNextReadSize(n int)
	SetPoller(pd Poller)
}

type Reactor func(Conn, []byte, error)
