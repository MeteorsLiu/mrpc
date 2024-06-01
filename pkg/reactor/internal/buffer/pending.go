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
	pb.buf = mempool.Get(len(buf))
	copy(pb.buf.B, buf)
	return pb
}

func (pb *PendingBuffer) SetPos(n int) {
	pb.pos += n
}

func (pb *PendingBuffer) Bytes() []byte {
	return pb.buf.B[pb.pos:]
}

func (pb *PendingBuffer) Wait() (int, error) {
	ret := <-pb.wake
	return ret.n, ret.err
}

func (pb *PendingBuffer) Release(n int, err error) {
	select {
	case pb.wake <- newResult(n, err):
	default:
	}
	mempool.Put(pb.buf)
	pb.buf = nil
}
