package event

import (
	"sync"
	"testing"
	"time"
)

func newTestQuota() *MonthlyQuota {
	now := time.Now()
	return &MonthlyQuota{
		counts: make(map[int64]int64),
		year:   now.Year(),
		month:  int(now.Month()),
		stopCh: make(chan struct{}),
	}
}

// TestQuotaTryIncrementRespectsMaxUnderConcurrency 验证并发下 TryIncrement 不会超发配额：
// 恰好只有 max 次返回 true，计数精确停在 max。
func TestQuotaTryIncrementRespectsMaxUnderConcurrency(t *testing.T) {
	q := newTestQuota()
	const userID = int64(1)
	const max = int64(100)
	const attempts = 1000

	var wg sync.WaitGroup
	var granted int64
	var mu sync.Mutex
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if q.TryIncrement(userID, max) {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if granted != max {
		t.Fatalf("expected exactly %d grants, got %d", max, granted)
	}
	if got := q.Get(userID); got != max {
		t.Fatalf("expected count %d, got %d", max, got)
	}
}

// TestQuotaTryIncrementUnlimited 验证 max<0 时始终递增。
func TestQuotaTryIncrementUnlimited(t *testing.T) {
	q := newTestQuota()
	for i := 0; i < 5; i++ {
		if !q.TryIncrement(2, -1) {
			t.Fatal("unlimited quota must always grant")
		}
	}
	if got := q.Get(2); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
}

func TestQuotaGetReturnsZeroForNewUser(t *testing.T) {
	q := newTestQuota()
	if got := q.Get(1); got != 0 {
		t.Fatalf("expected initial count 0, got %d", got)
	}
}

func TestQuotaIncrementIncreasesCount(t *testing.T) {
	q := newTestQuota()
	q.Increment(1)
	q.Increment(1)
	q.Increment(1)
	if got := q.Get(1); got != 3 {
		t.Fatalf("expected count 3, got %d", got)
	}
}

func TestQuotaIncrementReturnsNewValue(t *testing.T) {
	q := newTestQuota()
	for want := int64(1); want <= 5; want++ {
		if got := q.Increment(99); got != want {
			t.Fatalf("Increment #%d returned %d, want %d", want, got, want)
		}
	}
}

func TestQuotaCountersAreIndependentPerUser(t *testing.T) {
	q := newTestQuota()
	for userID := int64(1); userID <= 5; userID++ {
		for j := int64(0); j < userID; j++ {
			q.Increment(userID)
		}
	}
	for userID := int64(1); userID <= 5; userID++ {
		if got := q.Get(userID); got != userID {
			t.Errorf("user %d: expected count %d, got %d", userID, userID, got)
		}
	}
}

func TestQuotaConcurrentIncrementIsCorrect(t *testing.T) {
	q := newTestQuota()

	const goroutines = 200
	const userID = int64(1)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Increment(userID)
		}()
	}
	wg.Wait()

	if got := q.Get(userID); got != goroutines {
		t.Fatalf("expected count %d after %d concurrent increments, got %d", goroutines, goroutines, got)
	}
}

// TestQuotaStopExitsFlusherGoroutine verifies the fix: the background flusher
// goroutine must exit cleanly when Stop() closes stopCh.
func TestQuotaStopExitsFlusherGoroutine(t *testing.T) {
	q := newTestQuota()

	done := make(chan struct{})
	go func() {
		defer close(done)
		q.startFlusher()
	}()

	q.Stop()

	select {
	case <-done:
		// flusher exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("startFlusher did not exit after Stop() — goroutine leak")
	}
}

// TestQuotaConcurrentGetAndIncrement verifies that Get never races with Increment.
// Run with -race to detect data races.
func TestQuotaConcurrentGetAndIncrement(t *testing.T) {
	q := newTestQuota()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			q.Increment(1)
		}()
		go func() {
			defer wg.Done()
			_ = q.Get(1)
		}()
	}
	wg.Wait()
}
