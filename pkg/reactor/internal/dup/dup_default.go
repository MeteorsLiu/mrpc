//go:build !unix && !darwin

package dup

import (
	"net"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
)

func DupConn(cn net.Conn) (newFD int, err error) {
	err = reactor.ErrPlatformUnsupport
	return
}
