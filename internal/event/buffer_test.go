package event

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
)

// TestMain absorbs the ClickHouse connection panic that fires on the first
// repository initialisation attempt. After this, GetEventsRepository() returns
// nil safely and writeBatch returns "not initialized" errors instead of panicking.
func TestMain(m *testing.M) {
	func() {
		defer func() { recover() }()
		_ = repository.GetEventsRepository()
	}()
	os.Exit(m.Run())
}

// TestWriteBufferRejectsAddsAfterShutdown verifies the closed-flag fix:
// Add called after Shutdown must be a no-op (no wg.Add call, no buffer append).
func TestWriteBufferRejectsAddsAfterShutdown(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 100, time.Minute)
	wb.Start()
	wb.Shutdown()

	wb.Add(&models.Events{Name: "pageview"})
	wb.Add(&models.Events{Name: "pageview"})

	wb.mu.Lock()
	n := len(wb.buffer)
	wb.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected 0 buffered items after closed=1, got %d", n)
	}
}

// TestWriteBufferClosedFlagRejectsAddsImmediately tests the closed field directly.
func TestWriteBufferClosedFlagRejectsAddsImmediately(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 100, time.Minute)
	atomic.StoreInt32(&wb.closed, 1)

	wb.Add(&models.Events{Name: "pageview"})

	wb.mu.Lock()
	n := len(wb.buffer)
	wb.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected Add to be rejected when closed=1, got %d buffered items", n)
	}
}

// TestWriteBufferShutdownCompletesCleanly verifies that Shutdown returns without
// deadlocking after concurrent Add calls.
func TestWriteBufferShutdownCompletesCleanly(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 5, 10*time.Millisecond)
	wb.Start()

	var addWg sync.WaitGroup
	for i := 0; i < 10; i++ {
		addWg.Add(1)
		go func() {
			defer addWg.Done()
			wb.Add(&models.Events{Name: "pageview"})
		}()
	}
	addWg.Wait()

	done := make(chan struct{})
	go func() {
		defer close(done)
		wb.Shutdown()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not complete — possible WaitGroup deadlock")
	}
}

// TestWriteBufferDrainsOnSizeTrigger verifies that size-triggered flushes drain
// the in-memory buffer (writeBatch logs an error but buffer is emptied first).
func TestWriteBufferDrainsOnSizeTrigger(t *testing.T) {
	const batchSize = 3
	wb := NewWriteBuffer(context.Background(), batchSize, time.Minute)
	wb.Start()

	for i := 0; i < batchSize; i++ {
		wb.Add(&models.Events{Name: "pageview"})
	}

	// Shutdown waits for all flush goroutines to complete.
	wb.Shutdown()

	wb.mu.Lock()
	remaining := len(wb.buffer)
	wb.mu.Unlock()

	if remaining != 0 {
		t.Fatalf("expected empty buffer after shutdown, got %d items", remaining)
	}
}

// TestWriteBufferWriteBatchReturnsErrorForNilRepo verifies the nil-repository
// guard: writeBatch returns an error rather than panicking.
// (TestMain has pre-absorbed the ClickHouse connection panic so GetEventsRepository
// now returns nil safely.)
func TestWriteBufferWriteBatchReturnsErrorForNilRepo(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 10, time.Minute)
	err := wb.writeBatch([]*models.Events{{Name: "test"}})
	if err == nil {
		t.Fatal("expected error when events repository is not initialized, got nil")
	}
}

// TestWriteBufferClosedFlagPreventsWaitGroupRace verifies that after Shutdown
// sets closed=1 and wg.Wait() returns, subsequent Add calls cannot call wg.Add.
// Run with -race to surface WaitGroup concurrent-use panics.
func TestWriteBufferClosedFlagPreventsWaitGroupRace(t *testing.T) {
	for i := 0; i < 10; i++ {
		wb := NewWriteBuffer(context.Background(), 2, time.Minute)
		wb.Start()

		var wg sync.WaitGroup
		for j := 0; j < 4; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				wb.Add(&models.Events{Name: "pageview"})
			}()
		}
		wg.Wait()
		wb.Shutdown()
	}
}

// TestWriteBufferIntervalFlush verifies the ticker-driven flush path drains the buffer.
func TestWriteBufferIntervalFlush(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 1000, 20*time.Millisecond)
	wb.Start()

	for i := 0; i < 5; i++ {
		wb.Add(&models.Events{Name: "pageview"})
	}

	// Wait for at least one ticker-driven flush.
	time.Sleep(80 * time.Millisecond)

	wb.mu.Lock()
	remaining := len(wb.buffer)
	wb.mu.Unlock()

	if remaining != 0 {
		t.Fatalf("expected empty buffer after interval flush, got %d items", remaining)
	}

	wb.Shutdown()
}

// TestWriteBufferAdjustsBatchSize verifies the dynamic batch-size tuning path
// executes without panicking.
func TestWriteBufferAdjustsBatchSize(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 50, time.Minute)
	wb.Start()

	// Force the adjustBatchSize path (triggered at every 1000th event).
	atomic.StoreInt64(&wb.eventCounter, 999)
	wb.Add(&models.Events{Name: "pageview"}) // 1000th event triggers goroutine

	time.Sleep(30 * time.Millisecond)

	size := atomic.LoadInt32(&wb.batchSize)
	if size <= 0 {
		t.Fatalf("expected positive batchSize after adjustment, got %d", size)
	}

	wb.Shutdown()
}

// TestWriteBufferAddAppendsToBuffer verifies basic in-memory append behaviour.
func TestWriteBufferAddAppendsToBuffer(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 100, time.Minute)
	// Don't call Start — no ticker, no flush goroutine.

	for i := 0; i < 5; i++ {
		wb.Add(&models.Events{Name: "pageview"})
	}

	wb.mu.Lock()
	n := len(wb.buffer)
	wb.mu.Unlock()
	if n != 5 {
		t.Fatalf("expected 5 buffered items, got %d", n)
	}

	// Clean up without calling Shutdown (buffer is non-empty, which is fine
	// here because closed=1 stops the flush path).
	atomic.StoreInt32(&wb.closed, 1)
	wb.cancel()
}
