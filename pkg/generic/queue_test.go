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
