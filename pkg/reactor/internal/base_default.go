//go:build !unix && !darwin

package internal

import (
	"io"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
)

type BaseConn struct{}

func NewBaseConn(
	conn io.ReadWriteCloser,
	onread reactor.Reader,
	ondisconnect reactor.Disconnector,
) (*BaseConn, error) {
	return nil, reactor.ErrPlatformUnsupport
}
