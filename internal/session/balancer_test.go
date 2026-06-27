package session

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBalancerSerializesTasksForSameUserID verifies that tasks for the same
// userID are never executed concurrently (per-user serialization guarantee).
func TestBalancerSerializesTasksForSameUserID(t *testing.T) {
	b := NewBalancer[int]()

	const userID = uint64(42)
	var concurrent int32
	var maxConcurrent int32

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.Dispatch(userID, func() (int, error) {
				cur := atomic.AddInt32(&concurrent, 1)
				for {
					old := atomic.LoadInt32(&maxConcurrent)
					if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				atomic.AddInt32(&concurrent, -1)
				return i, nil
			}, 10*time.Second)
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&maxConcurrent); got > 1 {
		t.Fatalf("same-userID tasks ran concurrently (max observed concurrency = %d, want 1)", got)
	}
}

// TestBalancerDifferentUserIDsRunConcurrently verifies that tasks for different
// userIDs are dispatched to separate workers and can execute in parallel.
func TestBalancerDifferentUserIDsRunConcurrently(t *testing.T) {
	b := NewBalancer[int]()

	start := make(chan struct{})
	var reached int32
	const n = 4

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		userID := uint64(i) // different userIDs → different workers
		go func() {
			defer wg.Done()
			b.Dispatch(userID, func() (int, error) {
				atomic.AddInt32(&reached, 1)
				<-start
				return 0, nil
			}, 5*time.Second)
		}()
	}

	// All n workers must be running simultaneously; if they block on the same
	// worker queue this deadline fires before all n reach the rendezvous.
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	for atomic.LoadInt32(&reached) < n {
		select {
		case <-deadline.C:
			t.Fatalf("only %d/%d tasks started within 2s — different userIDs must run concurrently",
				atomic.LoadInt32(&reached), n)
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(start)
	wg.Wait()
}

// TestBalancerTimeoutReturnsDeadlineExceeded verifies that Dispatch returns
// context.DeadlineExceeded when the worker queue is full, without panicking.
func TestBalancerTimeoutReturnsDeadlineExceeded(t *testing.T) {
	b := NewBalancer[int]()

	// Fill the specific worker queue (capacity 64) with slow tasks.
	const userID = uint64(1)
	workerIdx := userID % uint64(balancerSize)
	blocker := make(chan struct{})
	for i := 0; i < 64; i++ {
		b.workers[workerIdx] <- balancerTask[int]{
			fn:     func() (int, error) { <-blocker; return 0, nil },
			result: make(chan balancerResult[int], 1),
		}
	}

	_, err := b.Dispatch(userID, func() (int, error) { return 99, nil }, 50*time.Millisecond)
	close(blocker)

	if err == nil {
		t.Fatal("expected DeadlineExceeded when worker queue is full, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

// TestBalancerDispatchReturnsResult verifies the happy path: result is delivered
// correctly through the worker→result channel pipeline.
func TestBalancerDispatchReturnsResult(t *testing.T) {
	b := NewBalancer[int]()

	got, err := b.Dispatch(1, func() (int, error) { return 42, nil }, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected result 42, got %d", got)
	}
}
