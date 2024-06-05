//go:build unix || darwin

package internal

import (
	"bytes"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/MeteorsLiu/mrpc/pkg/queue"
	"github.com/MeteorsLiu/mrpc/pkg/reactor"
	"github.com/MeteorsLiu/mrpc/pkg/reactor/internal/buffer"
	"github.com/MeteorsLiu/mrpc/pkg/reactor/internal/dup"
	"golang.org/x/sys/unix"
)

type BaseConn struct {
	fd int
	pd reactor.Poller

	wmu sync.RWMutex

	closed       atomic.Bool
	closeOnce    sync.Once
	laddr, raddr net.Addr

	isPinned    bool
	pinBuffer   *bytes.Buffer
	nextMinRead int

	onread       reactor.Reader
	ondisconnect reactor.Disconnector

	writePending *queue.Queue[*buffer.PendingBuffer]
}

func defaultReader(_ reactor.Conn, _ []byte)             {}
func defaultConnector(_ reactor.Conn, _ []byte, _ error) {}

func NewBaseConn(
	conn io.ReadWriteCloser,
	onread reactor.Reader,
	ondisconnect reactor.Disconnector,
) (*BaseConn, error) {
	if onread == nil {
		onread = defaultReader
	}
	if ondisconnect == nil {
		ondisconnect = defaultConnector
	}
	b := &BaseConn{
		onread:       onread,
		ondisconnect: ondisconnect,
		writePending: queue.New[*buffer.PendingBuffer](),
	}
	newFd, err := dup.DupConn(conn)
	if err != nil {
		return nil, err
	}
	b.setFD(newFd)
	b.setAddr(conn)

	runtime.SetFinalizer(b, (*BaseConn).Close)

	if err := Open(b); err != nil {
		b.Close()
		return nil, err
	}

	return b, nil
}

func (b *BaseConn) setAddr(conn io.ReadWriteCloser) {
	cn, ok := conn.(net.Conn)
	if !ok {
		return
	}
	b.raddr = cn.RemoteAddr()
	b.laddr = cn.LocalAddr()
}

func (b *BaseConn) setFD(fd int) {
	b.fd = fd
}

func (b *BaseConn) releasePinBuffer() {
	if b.pinBuffer != nil {
		buffer.PutPinBuffer(b.pinBuffer)
		b.pinBuffer = nil
	}
}

func (b *BaseConn) resetNextRead() {
	n := b.nextMinRead
	b.nextMinRead = 0
	if n > 0 {
		unix.SetsockoptInt(b.fd, unix.SOL_SOCKET, unix.SO_RCVLOWAT, 1)
		b.releasePinBuffer()
	}
}

func (b *BaseConn) rawRead(buf []byte) (n int, err error) {
	for {
		n, err = syscall.Read(b.fd, buf)
		if n == 0 && err == nil {
			err = io.EOF
		}
		if err != syscall.EINTR {
			if err == syscall.EAGAIN {
				err = nil
			}
			// when read returns errors, n = -1
			// filter it out.
			n = max(n, 0)
			break
		}
	}
	return
}

func (b *BaseConn) rawWrite(buf []byte) (nw int, err error) {
	var n int
	for nw < len(buf) {
		n, err = syscall.Write(b.fd, buf[nw:])
		if n > 0 {
			nw += n
		}
		// ignore EINTR
		if err != nil && err != syscall.EINTR {
			break
		}
		// special case, n == 0 and error == nil
		if n == 0 {
			err = io.ErrUnexpectedEOF
			break
		}
	}
	return
}

func (b *BaseConn) writeAllPending() (err error) {
	if b.closed.Load() || b.writePending.Len() == 0 {
		return
	}
	b.writePending.ForEach(func(pb *buffer.PendingBuffer) bool {
		var n int
		n, err = b.rawWrite(pb.Bytes())
		if err == syscall.EAGAIN && pb.Size() > n {
			pb.Consume(n)
			err = nil
			return false
		}
		// n == size, but with EAGAIN.
		if err == syscall.EAGAIN {
			err = nil
		}
		pb.Release(n, err)
		return true
	})
	return
}

func (b *BaseConn) releasePending() {
	b.writePending.Close()
	b.writePending.ForEach(func(pb *buffer.PendingBuffer) bool {
		pb.Release(0, net.ErrClosed)
		return true
	})
}

func (b *BaseConn) tryReadPinBuffer(buf []byte) (int, bool) {
	if b.pinBuffer == nil {
		return 0, false
	}
	nextRead := b.nextMinRead
	if nextRead == 0 && b.isPinned {
		b.pinBuffer.Write(buf)
		return -1, true
	}
	if nextRead == 0 {
		return 0, false
	}
	if nextRead > len(buf) {
		n, _ := b.pinBuffer.Write(buf)
		b.nextMinRead -= n
		return 0, false
	}
	n, _ := b.pinBuffer.Write(buf[:nextRead])
	return n, true
}

func (b *BaseConn) SetPoller(pd reactor.Poller) {
	b.pd = pd
}

func (b *BaseConn) FD() int {
	return b.fd
}

func (b *BaseConn) SetNextReadSize(n int) {
	if n > 0 {
		b.nextMinRead = n
		unix.SetsockoptInt(b.fd, unix.SOL_SOCKET, unix.SO_RCVLOWAT, n)
	}
}

func (b *BaseConn) Pin() bool {
	if b.isPinned {
		return false
	}
	b.isPinned = true
	return true
}

func (b *BaseConn) Unpin() bool {
	if b.pinBuffer == nil {
		return false
	}
	b.releasePinBuffer()
	b.isPinned = false
	return true
}

func (b *BaseConn) OnRead(buf []byte) {
	if n, ok := b.tryReadPinBuffer(buf); ok {
		b.onread(b, b.pinBuffer.Bytes())
		if n > 0 {
			b.resetNextRead()
			buf = buf[n:]
		} else {
			// when n <= 0, it means user pin the buffer
			// truncate the input buffer to prevent executing OnRead again.
			buf = buf[:0]
		}
	}
	if b.pinBuffer != nil || len(buf) == 0 {
		return
	}
	b.onread(b, buf)

	if b.isPinned || b.nextMinRead > 0 {
		b.pinBuffer = buffer.GetPinBuffer()
		b.pinBuffer.Write(buf)
	}
}

func (b *BaseConn) OnDisconnect(buf []byte, err error) {
	if b.pinBuffer != nil {
		if len(buf) > 0 {
			b.pinBuffer.Write(buf)
		}
		buf = b.pinBuffer.Bytes()
		defer b.releasePinBuffer()
	}
	if !b.closed.Load() {
		b.ondisconnect(b, buf, err)
	}
	b.Close()
}

// Unimplemented
func (b *BaseConn) Read(buf []byte) (n int, err error) {
	return 0, reactor.ErrProtocolUnsupport
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (b *BaseConn) Write(buf []byte) (n int, err error) {
	if b.closed.Load() {
		err = net.ErrClosed
		return
	}
	// this is not a typo
	// Write() will be only called in epoll thread,
	// there's no race, the rwlock is for wrapper.
	// wrapper may write concurrently.
	b.wmu.RLock()
	defer b.wmu.RUnlock()

	if b.writePending.Len() > 0 {
		// guarantee writing order
		b.writePending.Push(buffer.NewPendingBuffer(buf))
		return
	}
	n, err = b.rawWrite(buf)
	if err == syscall.EAGAIN && len(buf) > n {
		b.writePending.Push(buffer.NewPendingBuffer(buf[n:]))
	}
	return
}

// For wrapper
func (b *BaseConn) WriteBuffer(buf []byte) (n int, pb *buffer.PendingBuffer, err error) {
	if b.closed.Load() {
		err = net.ErrClosed
		return
	}
	b.wmu.Lock()
	defer b.wmu.Unlock()

	if b.writePending.Len() > 0 {
		// guarantee writing order
		pb = buffer.NewPendingBuffer(buf)
		b.writePending.Push(pb)
		return
	}
	n, err = b.rawWrite(buf)
	if err == syscall.EAGAIN && len(buf) > n {
		pb = buffer.NewPendingBuffer(buf[n:])
		b.writePending.Push(pb)
	}
	return
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (b *BaseConn) Close() error {
	if b.closed.Load() {
		return net.ErrClosed
	}
	var err error
	b.closeOnce.Do(func() {
		b.releasePending()
		runtime.SetFinalizer(b, nil)
		if b.pd != nil {
			err = b.pd.Close(b)
		}
		syscall.Close(b.fd)
		b.closed.Store(true)
	})
	return err
}

// LocalAddr returns the local network address, if known.
func (b *BaseConn) LocalAddr() net.Addr {
	return b.laddr
}

// RemoteAddr returns the remote network address, if known.
func (b *BaseConn) RemoteAddr() net.Addr {
	return b.raddr
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
func (b *BaseConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (b *BaseConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (b *BaseConn) SetWriteDeadline(t time.Time) error {
	return nil
}
