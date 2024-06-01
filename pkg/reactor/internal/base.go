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

	"github.com/MeteorsLiu/mrpc/pkg/condqueue"
	"github.com/MeteorsLiu/mrpc/pkg/reactor"
	"github.com/MeteorsLiu/mrpc/pkg/reactor/internal/buffer"
	"github.com/MeteorsLiu/mrpc/pkg/reactor/internal/dup"
	"golang.org/x/sys/unix"
)

type BaseConn struct {
	fd   int
	pd   reactor.Poller
	fdMu sync.RWMutex

	closed      bool
	pinBuffer   *bytes.Buffer
	nextMinRead atomic.Int32

	writePending *condqueue.CondQueue[*buffer.PendingBuffer]

	onread, ondisconnect reactor.Reactor
}

func defaultOnAction(_ reactor.Conn, _ []byte, _ error) {}

func NewBaseConn(conn io.ReadWriteCloser, onread, ondisconnect reactor.Reactor) (reactor.Conn, error) {
	if onread == nil {
		onread = defaultOnAction
	}
	if ondisconnect == nil {
		ondisconnect = defaultOnAction
	}
	b := &BaseConn{
		onread:       onread,
		ondisconnect: ondisconnect,
		writePending: condqueue.NewCondQueue[*buffer.PendingBuffer](),
	}
	newFd, err := dup.DupConn(conn)
	if err != nil {
		return nil, err
	}
	b.setFD(newFd)

	runtime.SetFinalizer(b, (*BaseConn).Close)

	if err := Open(b); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

func (b *BaseConn) setFD(fd int) {
	b.fd = fd
}

func (b *BaseConn) SetPoller(pd reactor.Poller) {
	b.pd = pd
}

func (b *BaseConn) FD() int {
	return b.fd
}

func (b *BaseConn) tryReadPinBuffer(buf []byte) (int, bool) {
	nextRead := int(b.nextMinRead.Load())
	if nextRead == 0 || b.pinBuffer == nil {
		return 0, false
	}
	if nextRead > len(buf) {
		b.pinBuffer.Write(buf)
		return 0, false
	}
	n, _ := b.pinBuffer.Write(buf[:nextRead])
	return n, true
}

func (b *BaseConn) OnRead(buf []byte, err error) {
	b.fdMu.RLock()
	defer b.fdMu.RUnlock()
	if n, ok := b.tryReadPinBuffer(buf); ok {
		b.onread(b, b.pinBuffer.Bytes(), err)
		b.resetNextRead()
		buf = buf[n:]
	}
	if b.pinBuffer != nil || len(buf) == 0 {
		return
	}
	if b.closed {
		b.onread(b, buf, net.ErrClosed)
		return
	}
	b.onread(b, buf, err)

	if b.nextMinRead.Load() > 0 {
		b.pinBuffer = buffer.GetPinBuffer()
		b.pinBuffer.Write(buf)
	}
}

func (b *BaseConn) OnDisconnect(buf []byte, err error) {
	b.fdMu.RLock()
	defer b.fdMu.RUnlock()
	if b.closed {
		b.ondisconnect(b, buf, net.ErrClosed)
		return
	}
	b.ondisconnect(b, buf, err)
}

func (b *BaseConn) SetNextReadSize(n int) {
	if n > 0 {
		b.nextMinRead.Store(int32(n))
		unix.SetsockoptInt(b.fd, unix.SOL_SOCKET, unix.SO_RCVLOWAT, n)
	}
}

func (b *BaseConn) resetNextRead() {
	if n := b.nextMinRead.Swap(0); n > 0 {
		unix.SetsockoptInt(b.fd, unix.SOL_SOCKET, unix.SO_RCVLOWAT, 1)
		buffer.PutPinBuffer(b.pinBuffer)
		b.pinBuffer = nil
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
			break
		}
	}
	return
}

func (b *BaseConn) rawWrite(buf []byte) (n int, err error) {
	for {
		n, err = syscall.Write(b.fd, buf)
		if n == 0 && err == nil {
			err = io.ErrUnexpectedEOF
		}
		if err != syscall.EINTR {
			break
		}
	}
	return
}

func (b *BaseConn) writeAllPending() {
	b.fdMu.RLock()
	defer b.fdMu.RUnlock()
	if b.closed {
		return
	}
	b.writePending.ForEach(func(pb *buffer.PendingBuffer) bool {
	retry:
		n, err := b.rawWrite(pb.Bytes())
		if err == syscall.EAGAIN && len(pb.Bytes()) > n {
			pb.SetPos(n)
			return false
		}
		if len(pb.Bytes()) > n && err == nil {
			// will this cause?
			// retry
			goto retry
		}
		pb.Release(n, err)
		return true
	}) ,,
}

func (b *BaseConn) releasePending() {
	b.writePending.Close()
	b.writePending.ForEach(func(pb *buffer.PendingBuffer) bool {
		pb.Release(0, net.ErrClosed)
		return true
	})
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (b *BaseConn) Read(buf []byte) (n int, err error) {

}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (b *BaseConn) Write(buf []byte) (n int, err error) {
	if b.closed {
		err = net.ErrClosed
		return
	}
	n, err = b.rawWrite(buf)
	if err == syscall.EAGAIN && len(buf) > n {
		b.writePending.Push(buffer.NewPendingBuffer(buf[n:]))
	}
	return
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (b *BaseConn) Close() error {
	b.fdMu.Lock()
	defer b.fdMu.Unlock()
	if b.closed {
		return net.ErrClosed
	}
	b.releasePending()
	var err error
	runtime.SetFinalizer(b, nil)
	if b.pd != nil {
		err = b.pd.Close(b)
	}
	syscall.Close(b.fd)
	b.closed = true
	return err
}

// LocalAddr returns the local network address, if known.
func (b *BaseConn) LocalAddr() net.Addr {

}

// RemoteAddr returns the remote network address, if known.
func (b *BaseConn) RemoteAddr() net.Addr {

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

}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (b *BaseConn) SetReadDeadline(t time.Time) error {

}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (b *BaseConn) SetWriteDeadline(t time.Time) error {

}
