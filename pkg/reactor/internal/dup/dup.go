//go:build unix

package dup

import (
	"io"
	"syscall"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
)

func DupConn(cn io.ReadWriteCloser) (newFD int, err error) {
	sc, ok := cn.(syscall.Conn)
	if !ok {
		err = reactor.ErrProtocolUnsupport
		return
	}
	ctl, err := sc.SyscallConn()
	if err != nil {
		return
	}
	ctl.Control(func(fd uintptr) {
		newFD, err = syscall.Dup(int(fd))
	})
	if err == nil {
		syscall.CloseOnExec(newFD)
		syscall.SetNonblock(newFD, true)
		cn.Close()
	}
	return
}
