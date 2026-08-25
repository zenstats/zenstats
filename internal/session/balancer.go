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
//
// timeout 只作用于"入队"阶段：worker 队列满且在 timeout 内未腾出空位时返回
// context.DeadlineExceeded，此时 fn 未执行、无任何副作用，调用方可安全丢弃/重试。
//
// 一旦任务成功入队，Dispatch 会一直阻塞到 fn 执行完毕并返回其结果，不再设第二次超时。
// 原因：fn（session 聚合）一定会在 worker 中执行并修改 session 状态（events+1 / pageviews+1
// 并写入 writeBuffer）；若此处提前超时返回 error，调用方会认为事件处理失败而不写 events 表，
// 造成 sessions 与 events 两张表计数不一致。fn 是有界工作（内存操作 + 最多一次带 2s 超时的
// CH 点查），所以等待其完成是有限的，不会造成无界阻塞。
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

	// Enqueued: fn will run exactly once; wait for its result so caller and worker agree.
	r := <-res
	return r.val, r.err
}
