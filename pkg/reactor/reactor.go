package reactor

import "net"

type Conn interface {
	net.Conn
	FD() int
	SetNextReadSize(n int)
	SetPoller(pd Poller)
	Pin() bool
	Unpin() bool
}

type Reader func(Conn, []byte)
type Disconnector func(Conn, []byte, error)
