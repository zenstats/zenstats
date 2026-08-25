package session

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
)

// TestMain absorbs the ClickHouse connection panic that fires on the first
// repository initialisation attempt. After this, GetSessionsRepository() returns
// nil safely and writeBatch returns "not initialized" errors instead of panicking.
func TestMain(m *testing.M) {
	func() {
		defer func() { recover() }()
		_ = repository.GetSessionsRepository()
	}()
	os.Exit(m.Run())
}

// TestSessionWriteBufferWriteBatchReturnsErrorForNilRepo verifies the nil-repository
// guard: writeBatch returns an error rather than panicking.
func TestSessionWriteBufferWriteBatchReturnsErrorForNilRepo(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 10, time.Minute)
	err := wb.writeBatch([]*models.Sessions{{Sign: 1}})
	if err == nil {
		t.Fatal("expected error when sessions repository is not initialized, got nil")
	}
}

// TestSessionWriteBufferShutdownDrainsAndCompletes verifies that Shutdown flushes
// remaining sessions and returns without deadlocking.
func TestSessionWriteBufferShutdownDrainsAndCompletes(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 100, time.Minute)
	wb.Start()

	for i := 0; i < 5; i++ {
		wb.Add(&models.Sessions{Sign: 1, UserId: uint64(i)})
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		wb.Shutdown()
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown timed out — possible deadlock")
	}

	wb.mu.Lock()
	remaining := len(wb.buffer)
	wb.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected empty buffer after Shutdown, got %d items", remaining)
	}
}

// TestSessionWriteBufferAddBelowBatchSizeDoesNotFlush verifies that adding fewer
// items than the batch threshold does not trigger an early flush.
func TestSessionWriteBufferAddBelowBatchSizeDoesNotFlush(t *testing.T) {
	const batchSize = 10
	wb := NewWriteBuffer(context.Background(), batchSize, time.Minute)
	wb.Start()

	for i := 0; i < batchSize-1; i++ {
		wb.Add(&models.Sessions{Sign: 1})
	}

	time.Sleep(20 * time.Millisecond)

	wb.mu.Lock()
	buffered := len(wb.buffer)
	wb.mu.Unlock()

	if buffered != batchSize-1 {
		t.Fatalf("expected %d items still buffered (no premature flush), got %d", batchSize-1, buffered)
	}

	wb.Shutdown()
}

// TestSessionWriteBufferSizeTriggerDrainsBuffer verifies that reaching the batch
// threshold spawns a flush that drains the buffer.
func TestSessionWriteBufferSizeTriggerDrainsBuffer(t *testing.T) {
	const batchSize = 4
	wb := NewWriteBuffer(context.Background(), batchSize, time.Minute)
	wb.Start()

	for i := 0; i < batchSize; i++ {
		wb.Add(&models.Sessions{Sign: 1})
	}

	wb.Shutdown()

	wb.mu.Lock()
	remaining := len(wb.buffer)
	wb.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected empty buffer after size-triggered flush + shutdown, got %d", remaining)
	}
}

// TestSessionWriteBufferFindPendingReturnsLatestState verifies FindPending picks the
// highest-version Sign=+1 row for the user/site, ignoring tombstones, other users and
// rows older than notBefore (e.g. historical sessions passing through the buffer).
func TestSessionWriteBufferFindPendingReturnsLatestState(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 100, time.Minute)
	now := time.Now()

	wb.Add(&models.Sessions{Sign: 1, Version: 10, UserId: 1, SiteId: 1, Timestamp: now, Events: 1})
	wb.Add(&models.Sessions{Sign: -1, Version: 10, UserId: 1, SiteId: 1, Timestamp: now, Events: 1})
	wb.Add(&models.Sessions{Sign: 1, Version: 11, UserId: 1, SiteId: 1, Timestamp: now, Events: 2})
	wb.Add(&models.Sessions{Sign: 1, Version: 99, UserId: 2, SiteId: 1, Timestamp: now, Events: 5})            // other user
	wb.Add(&models.Sessions{Sign: 1, Version: 999, UserId: 1, SiteId: 1, Timestamp: now.Add(-48 * time.Hour)}) // historical

	got := wb.FindPending(1, 1, now.Add(-time.Hour))
	if got == nil || got.Version != 11 || got.Events != 2 {
		t.Fatalf("expected pending row version=11 events=2, got %+v", got)
	}
	if wb.FindPending(3, 1, now.Add(-time.Hour)) != nil {
		t.Fatal("expected nil for unknown user")
	}
}

// TestSessionWriteBufferFindPendingSeesInflightBatch verifies rows taken out of the
// buffer by a flush remain discoverable while the ClickHouse write is in progress.
func TestSessionWriteBufferFindPendingSeesInflightBatch(t *testing.T) {
	wb := NewWriteBuffer(context.Background(), 100, time.Minute)
	now := time.Now()
	wb.Add(&models.Sessions{Sign: 1, Version: 5, UserId: 9, SiteId: 1, Timestamp: now})

	// Simulate flushAndWrite's first half: move buffer into an inflight batch.
	wb.mu.Lock()
	batch := wb.batchPool.Get().(*[]*models.Sessions)
	*batch = append(*batch, wb.buffer...)
	wb.buffer = wb.buffer[:0]
	wb.inflight[batch] = struct{}{}
	wb.mu.Unlock()

	if got := wb.FindPending(9, 1, now.Add(-time.Hour)); got == nil || got.Version != 5 {
		t.Fatalf("expected inflight row version=5, got %+v", got)
	}

	wb.mu.Lock()
	delete(wb.inflight, batch)
	wb.mu.Unlock()
	if wb.FindPending(9, 1, now.Add(-time.Hour)) != nil {
		t.Fatal("expected nil after inflight batch completed")
	}
}
