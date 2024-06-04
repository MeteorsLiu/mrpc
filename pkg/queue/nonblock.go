package queue

import (
	"fmt"

	"github.com/MeteorsLiu/mrpc/pkg/queue/internal"
)

// Queue is a producer wait free but consumer wait queue
// use cond instead of channel for resizeable queue.
type Queue[T any] struct {
	closed  bool
	buffers *internal.List[T]
}

func New[T any]() *Queue[T] {
	return &Queue[T]{
		buffers: internal.New[T](),
	}
}

func (n *Queue[T]) Len() int {
	return n.buffers.Len()
}

func (n *Queue[T]) Push(value T) {
	if !n.closed {
		n.buffers.PushBack(value)
	}
}

func (n *Queue[T]) Close() {
	n.closed = true
}

func (n *Queue[T]) ForEach(fn func(T) bool) {
	if n.buffers == nil {
		fmt.Println("empty")
		return
	}
	var next *internal.Element[T]
	for e := n.buffers.Front(); e != nil; e = next {
		next = e.Next()
		if !fn(e.Value) {
			break
		}
		n.buffers.Remove(e)
	}
}
