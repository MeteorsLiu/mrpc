package buffer

import (
	"bytes"
	"sync"
)

var (
	pinBuffersPool = sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
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
