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
	closed  bool

	// head 缓存一个已从 channel 取出的队首元素，用于实现非破坏性、保序的 Peek。
	// channel 无法在不移除元素的前提下查看队首，故 Peek 命中时把元素暂存于此，
	// 由后续 Dequeue/Peek 优先消费，从而保持 FIFO 顺序。
	hasHead bool
	head    T
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

	// 队列已关闭时拒绝入队，避免向已关闭的 channel 发送导致 panic
	if q.closed {
		return fmt.Errorf("queue is closed")
	}

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
	// 优先消费被 Peek 暂存的队首元素，保持 FIFO
	if q.hasHead {
		v := q.head
		var zero T
		q.head, q.hasHead = zero, false
		return v
	}
	select {
	case value := <-q.queue:
		return value
	default:
		var zero T
		return zero
	}
}

// Peek 非破坏性地查看队首元素而不将其移出队列。
// 返回 (元素, true) 表示存在队首元素；(零值, false) 表示队列为空（或已关闭且排空）。
// 元素被暂存于 head，后续 Dequeue/Peek 会优先返回它，因此不会打乱 FIFO 顺序，
// 也不会向 channel 回写（避免在队列关闭后向已关闭 channel 发送而 panic）。
func (q *DynamicQueue[T]) Peek() (T, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.hasHead {
		return q.head, true
	}
	select {
	case value, ok := <-q.queue:
		if !ok {
			// channel 已关闭且排空
			var zero T
			return zero, false
		}
		q.head, q.hasHead = value, true
		return value, true
	default:
		var zero T
		return zero, false
	}
}

func (q *DynamicQueue[T]) IsEmpty() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return !q.hasHead && len(q.queue) == 0
}

// IsClosed 返回队列是否已关闭。
func (q *DynamicQueue[T]) IsClosed() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.closed
}

func (q *DynamicQueue[T]) Size() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	n := len(q.queue)
	if q.hasHead {
		n++
	}
	return n
}

func (q *DynamicQueue[T]) Close() error {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.closed {
		return nil
	}
	q.closed = true
	close(q.queue)
	return nil
}
