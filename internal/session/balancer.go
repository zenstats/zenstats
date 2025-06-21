package session

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Balancer[T any] struct {
	locks *sync.Map
}

func NewBalancer[T any]() *Balancer[T] {
	return &Balancer[T]{
		locks: new(sync.Map),
	}
}

func (b *Balancer[T]) Dispatch(
	userID uint64,
	fn func() (T, error),
	timeout time.Duration,
) (res T, err error) {
	lockKey := fmt.Sprintf("session_lock:%d", userID)
	lock := b.acquireLock(lockKey)
	lock.Lock()
	defer lock.Unlock()

	// 执行处理函数
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		res, err = fn()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// 获取锁
func (b *Balancer[T]) acquireLock(key string) sync.Locker {
	actual, _ := b.locks.LoadOrStore(key, &sync.Mutex{})
	return actual.(sync.Locker)
}
