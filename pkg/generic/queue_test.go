package generic

import (
	"sync"
	"testing"
)

func TestIsEmptyReturnsTrueForNewQueue(t *testing.T) {
	q := NewQueue[string](8, 16)
	if !q.IsEmpty() {
		t.Fatal("expected IsEmpty() == true for a new queue")
	}
}

func TestSizeReturnsZeroForNewQueue(t *testing.T) {
	q := NewQueue[int](8, 16)
	if got := q.Size(); got != 0 {
		t.Fatalf("expected Size() == 0, got %d", got)
	}
}

func TestSizeAndIsEmptyReflectEnqueuedItems(t *testing.T) {
	q := NewQueue[int](8, 16)
	for i := 0; i < 5; i++ {
		if err := q.Enqueue(i); err != nil {
			t.Fatalf("Enqueue(%d): %v", i, err)
		}
	}
	if got := q.Size(); got != 5 {
		t.Fatalf("expected Size() == 5, got %d", got)
	}
	if q.IsEmpty() {
		t.Fatal("expected IsEmpty() == false after enqueuing 5 items")
	}
}

func TestIsEmptyAfterDequeueLastItem(t *testing.T) {
	q := NewQueue[int](4, 8)
	_ = q.Enqueue(42)
	q.Dequeue()
	if !q.IsEmpty() {
		t.Fatal("expected IsEmpty() == true after dequeuing the only item")
	}
	if got := q.Size(); got != 0 {
		t.Fatalf("expected Size() == 0 after dequeue, got %d", got)
	}
}

// TestIsEmptyAndSizeSafeUnderConcurrency verifies that IsEmpty and Size do not
// race with concurrent Enqueue/Dequeue. Run with -race to detect data races.
func TestIsEmptyAndSizeSafeUnderConcurrency(t *testing.T) {
	q := NewQueue[int](4, 256)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			_ = q.Enqueue(v)
			_ = q.IsEmpty()
			_ = q.Size()
			q.Dequeue()
			_ = q.IsEmpty()
			_ = q.Size()
		}(i)
	}
	wg.Wait()
}

func TestQueueGrowsBeyondInitialCapacity(t *testing.T) {
	q := NewQueue[int](2, 64)
	for i := 0; i < 10; i++ {
		if err := q.Enqueue(i); err != nil {
			t.Fatalf("Enqueue(%d): %v", i, err)
		}
	}
	if got := q.Size(); got != 10 {
		t.Fatalf("expected Size() == 10 after growth, got %d", got)
	}
}

func TestEnqueueFailsAtMaxSize(t *testing.T) {
	q := NewQueue[int](2, 2)
	_ = q.Enqueue(1)
	_ = q.Enqueue(2)
	if err := q.Enqueue(3); err == nil {
		t.Fatal("expected error when queue is full and at max capacity")
	}
}

// TestPeekPreservesFIFOOrder 验证 Peek 非破坏、保序：Peek 后 Dequeue 仍按 1,2,3 顺序返回。
func TestPeekPreservesFIFOOrder(t *testing.T) {
	q := NewQueue[int](4, 16)
	for _, v := range []int{1, 2, 3} {
		if err := q.Enqueue(v); err != nil {
			t.Fatalf("enqueue %d: %v", v, err)
		}
	}
	// 多次 Peek 应稳定返回队首 1，且不改变队列
	for i := 0; i < 3; i++ {
		if v, ok := q.Peek(); !ok || v != 1 {
			t.Fatalf("Peek #%d = (%d,%v), want (1,true)", i, v, ok)
		}
	}
	if q.Size() != 3 {
		t.Fatalf("Size after Peek = %d, want 3", q.Size())
	}
	// Dequeue 顺序不被 Peek 打乱
	for _, want := range []int{1, 2, 3} {
		if got := q.Dequeue(); got != want {
			t.Fatalf("Dequeue = %d, want %d", got, want)
		}
	}
}

// TestPeekEmptyQueue 验证空队列 Peek 返回 (零值, false)。
func TestPeekEmptyQueue(t *testing.T) {
	q := NewQueue[int](4, 16)
	if v, ok := q.Peek(); ok || v != 0 {
		t.Fatalf("Peek empty = (%d,%v), want (0,false)", v, ok)
	}
}

// TestPeekAfterCloseNoPanic 验证关闭后 Peek 不 panic：先返回已缓存队首，排空后返回 false。
func TestPeekAfterCloseNoPanic(t *testing.T) {
	q := NewQueue[int](4, 16)
	_ = q.Enqueue(42)
	if v, ok := q.Peek(); !ok || v != 42 { // 先缓存队首
		t.Fatalf("Peek before close = (%d,%v), want (42,true)", v, ok)
	}
	_ = q.Close()
	// 关闭后再次 Peek 不能向已关闭 channel 回写而 panic
	if v, ok := q.Peek(); !ok || v != 42 {
		t.Fatalf("Peek after close = (%d,%v), want (42,true)", v, ok)
	}
	if got := q.Dequeue(); got != 42 {
		t.Fatalf("Dequeue after close = %d, want 42", got)
	}
	if _, ok := q.Peek(); ok {
		t.Fatalf("Peek on drained+closed queue should be false")
	}
}
