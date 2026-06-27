package generic

import (
	"fmt"
	"log/slog"
	"sync"
)

type DynamicQueue[T any] struct {
	queue   chan T
	lock    sync.Mutex
	maxSize int
}

func NewQueue[T any](initialSize, maxSize int) *DynamicQueue[T] {
	return &DynamicQueue[T]{
		queue:   make(chan T, initialSize),
		maxSize: maxSize,
	}
}

// Enqueue 向动态队列中添加一个元素。
func (q *DynamicQueue[T]) Enqueue(item T) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	slog.Debug("Enqueue")

	select {
	case q.queue <- item:
		return nil
	default:
		if cap(q.queue) >= q.maxSize {
			return fmt.Errorf("queue is full and has reached max size %d", q.maxSize)
		}

		newSize := cap(q.queue) * 2
		if newSize > q.maxSize {
			newSize = q.maxSize
		}
		newCh := make(chan T, newSize)

		// 迁移数据
		close(q.queue)
		for v := range q.queue {
			newCh <- v
		}
		newCh <- item
		q.queue = newCh
		return nil
	}
}

// Dequeue 从动态队列中移除并返回一个元素
func (q *DynamicQueue[T]) Dequeue() T {
	q.lock.Lock()
	defer q.lock.Unlock()
	select {
	case value := <-q.queue:
		return value
	default:
		var zero T
		return zero
	}
}

func (q *DynamicQueue[T]) Peek() (T, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	select {
	case value := <-q.queue:
		q.queue <- value
		return value, true
	default:
		var zero T
		return zero, false
	}
}

func (q *DynamicQueue[T]) IsEmpty() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.queue) == 0
}

func (q *DynamicQueue[T]) Size() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.queue)
}

func (q *DynamicQueue[T]) Close() error {
	q.lock.Lock()
	defer q.lock.Unlock()
	close(q.queue)
	return nil
}
