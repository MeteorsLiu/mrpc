package conn

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
	"github.com/MeteorsLiu/mrpc/pkg/reactor/internal"
)

// conn is a wrapper of BaseConn
// In any time, We MUST NOT call function in *BaseConn directly
// because of data race. That's why baseconn is in internal package.
type conn struct {
	mu   sync.RWMutex
	base *internal.BaseConn
}

func NewConn(
	cn io.ReadWriteCloser,
	onread reactor.Reader,
	ondisconnect reactor.Disconnector,
) (net.Conn, error) {
	base, err := internal.NewBaseConn(cn, onread, ondisconnect)
	if err != nil {
		return nil, err
	}
	return &conn{base: base}, nil
}

func (b *conn) Read(buf []byte) (n int, err error) {
	return 0, reactor.ErrProtocolUnsupport
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (b *conn) Write(buf []byte) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	n, pb, err := b.base.WriteBuffer(buf)
	if pb != nil {
		n, err = pb.Wait()
	}
	return n, err
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (b *conn) Close() error {
	b.mu.Lock()
	err := b.base.Close()
	b.mu.Unlock()
	return err
}

// LocalAddr returns the local network address, if known.
func (b *conn) LocalAddr() net.Addr {
	return b.base.LocalAddr()
}

// RemoteAddr returns the remote network address, if known.
func (b *conn) RemoteAddr() net.Addr {
	return b.base.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (b *conn) SetDeadline(t time.Time) error {
	return b.base.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (b *conn) SetReadDeadline(t time.Time) error {
	return b.base.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (b *conn) SetWriteDeadline(t time.Time) error {
	return b.base.SetWriteDeadline(t)
}
