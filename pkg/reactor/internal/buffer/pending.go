package buffer

import (
	"github.com/MeteorsLiu/mempool"
)

type result struct {
	n   int
	err error
}

func newResult(n int, err error) *result {
	return &result{n, err}
}

type PendingBuffer struct {
	pos  int
	wake chan *result
	buf  *mempool.Buffer
}

func NewPendingBuffer(buf []byte) *PendingBuffer {
	pb := &PendingBuffer{
		wake: make(chan *result, 1),
	}
	if len(buf) > 0 {
		pb.buf = mempool.Get(len(buf))
		copy(pb.buf.B, buf)
	}
	return pb
}

func (pb *PendingBuffer) IsDone() bool {
	return pb.pos >= len(pb.buf.B)
}

func (pb *PendingBuffer) Consume(n int) {
	if pb.buf != nil && n > 0 {
		// bound check
		pb.pos = min(pb.pos+n, len(pb.buf.B))
	}
}

func (pb *PendingBuffer) Bytes() []byte {
	if pb.buf == nil {
		return nil
	}
	return pb.buf.B[pb.pos:]
}

func (pb *PendingBuffer) Wait() (int, error) {
	ret := <-pb.wake
	return ret.n, ret.err
}

func (pb *PendingBuffer) Size() int {
	return len(pb.Bytes())
}

func (pb *PendingBuffer) Release(n int, err error) {
	select {
	case pb.wake <- newResult(n, err):
	default:
	}
	if pb.buf != nil {
		mempool.Put(pb.buf)
		pb.buf = nil
	}
}
