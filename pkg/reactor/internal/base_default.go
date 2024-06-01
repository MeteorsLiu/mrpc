//go:build !unix && !darwin

package internal

import (
	"net"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
)

type BaseConn struct{}

func NewBaseConn(conn net.Conn) (reactor.Conn, error) {
	return nil, reactor.ErrPlatformUnsupport
}
