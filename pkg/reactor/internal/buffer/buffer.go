package buffer

import (
	"bytes"
	"sync"

	"github.com/MeteorsLiu/mrpc/pkg/reactor"
)

var (
	pinBuffersPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, reactor.MaxPacketSize))
		},
	}
)

func GetPinBuffer() *bytes.Buffer {
	return pinBuffersPool.Get().(*bytes.Buffer)
}

func PutPinBuffer(buf *bytes.Buffer) {
	buf.Reset()
	pinBuffersPool.Put(buf)
}
