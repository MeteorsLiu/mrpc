package queue

import (
	"github.com/MeteorsLiu/mrpc/pkg/queue/list"
)

// Queue is a producer wait free but consumer wait queue
// use cond instead of channel for resizeable queue.
type Queue[T any] struct {
	closed  bool
	buffers *list.List[T]
}

func New[T any]() *Queue[T] {
	return &Queue[T]{
		buffers: list.New[T](),
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
	var next *list.Element[T]
	for e := n.buffers.Front(); e != nil; e = next {
		next = e.Next()
		if !fn(e.Value) {
			break
		}
		n.buffers.Remove(e)
	}
}
