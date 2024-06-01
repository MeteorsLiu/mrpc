package reactor

type Poller interface {
	Open(Conn) error
	Close(Conn) error
}

const MaxPacketSize = 65536
