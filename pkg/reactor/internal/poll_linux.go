//go:build linux

package internal

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
	"golang.org/x/sys/unix"
)

type EpollEvent struct {
	Events uint32
	Data   [8]byte // unaligned uintptr
}

func Register() {
	for i := 0; i < runtime.NumCPU(); i++ {
		globalPoller = append(globalPoller, newEpoll())
	}
}

type epoll struct {
	epfd int
	buf  []byte
}

func EpollCtl(epfd, op, fd int, event *EpollEvent) (errno syscall.Errno) {
	_, _, e := syscall.Syscall6(
		syscall.SYS_EPOLL_CTL,
		uintptr(epfd),
		uintptr(op),
		uintptr(fd),
		uintptr(unsafe.Pointer(event)),
		0, 0)
	return e
}

func EpollWait(epfd int, events []EpollEvent, maxev, waitms int) (int, syscall.Errno) {
	var ev unsafe.Pointer
	var _zero uintptr
	if len(events) > 0 {
		ev = unsafe.Pointer(&events[0])
	} else {
		ev = unsafe.Pointer(&_zero)
	}
	r1, _, e := syscall.Syscall6(
		syscall.SYS_EPOLL_PWAIT,
		uintptr(epfd),
		uintptr(ev),
		uintptr(maxev),
		uintptr(waitms),
		0, 0)
	return int(r1), e
}

func newEpoll() reactor.Poller {
	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return nil
	}
	p := &epoll{epfd: epfd, buf: make([]byte, reactor.MaxPacketSize)}
	go p.run()
	return p
}

func (p *epoll) Open(r reactor.Conn) error {
	bc, ok := r.(*BaseConn)
	if !ok {
		return reactor.ErrProtocolUnsupport
	}
	var ev EpollEvent
	ev.Events = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLRDHUP | unix.EPOLLET
	*(**BaseConn)(unsafe.Pointer(&ev.Data)) = bc
	bc.SetPoller(p)
	err := EpollCtl(p.epfd, syscall.EPOLL_CTL_ADD, r.FD(), &ev)
	if err == 0 {
		return nil
	}
	return os.NewSyscallError("EpollCtl", err)
}

func (p *epoll) Close(r reactor.Conn) error {
	var ev EpollEvent
	r.SetPoller(nil)
	err := EpollCtl(p.epfd, syscall.EPOLL_CTL_DEL, r.FD(), &ev)
	if err == 0 {
		return nil
	}
	return os.NewSyscallError("EpollCtl", err)
}

func (p *epoll) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var events [128]EpollEvent

	for {
		n, err := EpollWait(p.epfd, events[:], len(events), -1)
		if err != 0 {
			if err == syscall.EINTR {
				continue
			}
			return
		}
		for i := 0; i < n; i++ {
			ev := events[i]
			conn := *(**BaseConn)(unsafe.Pointer(&ev.Data))
			// Order: If there's something to write, write it(because onread may call Write)
			// Then, read it.
			// Finally, handle the disconnected error.
			var er error
			if ev.Events&syscall.EPOLLOUT != 0 {
				er = conn.writeAllPending()
			}

			if ev.Events&syscall.EPOLLIN != 0 {
				n, er = conn.rawRead(p.buf)
				// ignore error, if there's something to read
				// read it.
				// the error will be handle in OnDisconnect
				if n > 0 {
					conn.OnRead(p.buf[:n])
				}
			}

			if er != nil || ev.Events&(syscall.EPOLLRDHUP|syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
				if er == nil {
					n, er = conn.rawRead(p.buf)
					if n > 0 {
						conn.OnDisconnect(p.buf[:n], er)
						continue
					}
				}
				conn.OnDisconnect(nil, er)
			}
		}
	}
}
