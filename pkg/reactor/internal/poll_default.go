package internal

import (
	"sync/atomic"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
)

var (
	seq          atomic.Uint64
	globalPoller []reactor.Poller
)

func init() {
	Register()
}

func Open(r reactor.Conn) error {
	if len(globalPoller) == 0 {
		return reactor.ErrPlatformUnsupport
	}
	pos := int(seq.Add(1)-1) % len(globalPoller)
	return globalPoller[pos].Open(r)
}
