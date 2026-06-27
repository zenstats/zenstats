package session

import (
	"context"
	"time"
)

// balancerSize 对齐 Plausible 生产配置：100 个固定 worker，通过 userID % size 路由。
// 同一用户的所有事件永远路由到同一 worker，天然串行，无需 per-user 锁。
const balancerSize = 100

type balancerTask[T any] struct {
	fn     func() (T, error)
	result chan balancerResult[T]
}

type balancerResult[T any] struct {
	val T
	err error
}

type Balancer[T any] struct {
	workers []chan balancerTask[T]
}

func NewBalancer[T any]() *Balancer[T] {
	b := &Balancer[T]{
		workers: make([]chan balancerTask[T], balancerSize),
	}
	for i := range b.workers {
		ch := make(chan balancerTask[T], 64)
		b.workers[i] = ch
		go b.runWorker(ch)
	}
	return b
}

func (b *Balancer[T]) runWorker(ch <-chan balancerTask[T]) {
	for t := range ch {
		val, err := t.fn()
		t.result <- balancerResult[T]{val: val, err: err}
	}
}

// Dispatch 将 fn 路由到与 userID 绑定的 worker 串行执行。
// 同一 userID 的调用永远由同一 worker 顺序处理，不会并发。
// timeout 超时后返回 error，但 fn 仍会在 worker 中完成执行（保证 session 数据一致性）。
func (b *Balancer[T]) Dispatch(userID uint64, fn func() (T, error), timeout time.Duration) (T, error) {
	idx := userID % uint64(balancerSize)
	res := make(chan balancerResult[T], 1)
	t := balancerTask[T]{fn: fn, result: res}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Block until the worker queue has room, or timeout. Direct-execute fallback was removed:
	// it bypassed per-userId serialization and could cause concurrent session mutations.
	select {
	case b.workers[idx] <- t:
	case <-timer.C:
		var zero T
		return zero, context.DeadlineExceeded
	}

	select {
	case r := <-res:
		return r.val, r.err
	case <-timer.C:
		// fn is still queued and will complete in the worker (data consistency preserved).
		var zero T
		return zero, context.DeadlineExceeded
	}
}
