package event

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
)

type WriteBuffer struct {
	mu            sync.Mutex
	wg            sync.WaitGroup
	batchSize     int32
	flushInterval time.Duration
	minBatchSize  int
	maxBatchSize  int
	eventCounter  int64
	writeTimes    []time.Duration
	closed        int32 // atomic; set to 1 by Shutdown to stop new flush goroutines

	buffer []*models.Events

	maxBufferSize int           // 缓冲上限：下游长时间不可用时超过则丢弃计数，避免无界增长 OOM
	dropped       int64         // atomic：因缓冲满而丢弃的事件计数
	flushSem      chan struct{} // 限制并发 flush goroutine 数量，防止下游阻塞时无界堆积

	shutdownChan chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc

	batchPool sync.Pool
}

// maxConcurrentFlushes 限制同时进行的 flush goroutine 数量。
const maxConcurrentFlushes = 8

func NewWriteBuffer(ctx context.Context, batchSize int, flushInterval time.Duration) *WriteBuffer {
	ctx, cancel := context.WithCancel(ctx)

	minBatchSize := batchSize / 2
	if minBatchSize < 50 {
		minBatchSize = 50
	}
	maxBatchSize := batchSize * 2
	// 保证 max >= 初始 batchSize，且 min <= max，否则自适应调整边界失效
	if maxBatchSize < batchSize {
		maxBatchSize = batchSize
	}
	if minBatchSize > maxBatchSize {
		minBatchSize = maxBatchSize
	}

	maxBufferSize := batchSize * 50
	if maxBufferSize < 50000 {
		maxBufferSize = 50000
	}

	wb := &WriteBuffer{
		batchSize:     int32(batchSize),
		minBatchSize:  minBatchSize,
		maxBatchSize:  maxBatchSize,
		maxBufferSize: maxBufferSize,
		flushSem:      make(chan struct{}, maxConcurrentFlushes),
		flushInterval: flushInterval,
		buffer:        make([]*models.Events, 0, batchSize),
		shutdownChan:  make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
		writeTimes:    make([]time.Duration, 0, 5),
	}
	wb.batchPool.New = func() any {
		s := make([]*models.Events, 0, int(atomic.LoadInt32(&wb.batchSize)))
		return &s
	}

	return wb
}

func (wb *WriteBuffer) Add(event *models.Events) {
	// Reject new events after Shutdown() is called to prevent wg.Add races.
	if atomic.LoadInt32(&wb.closed) == 1 {
		return
	}

	wb.mu.Lock()
	if len(wb.buffer) >= wb.maxBufferSize {
		wb.mu.Unlock()
		// 缓冲达到上限（通常是下游 ClickHouse 长时间不可用）：丢弃并计数，避免无界增长导致 OOM。
		n := atomic.AddInt64(&wb.dropped, 1)
		if n%1000 == 1 {
			slog.Error("event write buffer full, dropping events", "dropped_total", n, "max_buffer", wb.maxBufferSize)
		}
		return
	}
	wb.buffer = append(wb.buffer, event)
	shouldFlush := len(wb.buffer) >= int(atomic.LoadInt32(&wb.batchSize))
	wb.mu.Unlock()

	if atomic.AddInt64(&wb.eventCounter, 1)%1000 == 0 {
		go wb.adjustBatchSize()
	}
	slog.Debug("add event to buffer", "event", event)
	if shouldFlush {
		wb.spawnFlush()
	}
}

// spawnFlush 在并发未达上限时起一个独立 flush goroutine；达到上限则跳过，
// 交由 flushWorker 的定时器或后续 Add 处理，防止下游阻塞时 flush goroutine 无界堆积。
func (wb *WriteBuffer) spawnFlush() {
	select {
	case wb.flushSem <- struct{}{}:
		wb.wg.Add(1)
		go func() {
			defer wb.wg.Done()
			defer func() { <-wb.flushSem }()
			wb.flushAndWrite()
		}()
	default:
		// 并发已达上限：本次不新起 goroutine，交给 flushWorker 定时器或后续 Add 兜底，
		// 避免下游阻塞时 flush goroutine 无界堆积。
	}
}

// flushAndWrite 从 buffer 取出数据并直接写入 ClickHouse
// 多个 goroutine 并发调用是安全的：只有第一个能取到数据，其余见到空 buffer 直接返回。
func (wb *WriteBuffer) flushAndWrite() {
	wb.mu.Lock()
	if len(wb.buffer) == 0 {
		wb.mu.Unlock()
		return
	}
	batch := wb.batchPool.Get().(*[]*models.Events)
	*batch = append(*batch, wb.buffer...)
	wb.buffer = wb.buffer[:0]
	wb.mu.Unlock()

	if err := wb.writeBatch(*batch); err != nil {
		slog.Error("failed to write event batch, events dropped", "error", err, "dropped_count", len(*batch))
	}
	*batch = (*batch)[:0]
	wb.batchPool.Put(batch)
}

func (wb *WriteBuffer) Start() {
	wb.wg.Add(1)
	go wb.flushWorker()
}

func (wb *WriteBuffer) flushWorker() {
	defer wb.wg.Done()

	ticker := time.NewTicker(wb.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wb.flushAndWrite()

		case <-wb.shutdownChan:
			wb.flushAndWrite()
			return

		case <-wb.ctx.Done():
			wb.flushAndWrite()
			return
		}
	}
}

func (wb *WriteBuffer) writeBatch(batch []*models.Events) error {
	repo := repository.GetEventsRepository()
	if repo == nil {
		return fmt.Errorf("events repository not initialized")
	}

	retries := 5
	var err error
	backoff := []time.Duration{100 * time.Millisecond, 300 * time.Millisecond, 1 * time.Second, 3 * time.Second, 5 * time.Second}

	// 一次批次生成一个去重 token，所有重试复用它，保证"已提交但超时后重试"不重复计数。
	insertCtx := repository.WithInsertDedupToken(wb.ctx)

	startTime := time.Now()
	for i := 0; i < retries; i++ {
		if err = repo.BatchInsert(insertCtx, batch); err == nil {
			wb.recordWriteTime(time.Since(startTime))
			return nil
		}
		slog.Debug("failed to flush batch, retrying...", "retry", i+1, "error", err)
		select {
		case <-time.After(backoff[i]):
		case <-wb.ctx.Done():
			return wb.ctx.Err()
		}
	}
	return fmt.Errorf("failed after %d retries: %w", retries, err)
}

func (wb *WriteBuffer) recordWriteTime(duration time.Duration) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.writeTimes = append(wb.writeTimes, duration)
	if len(wb.writeTimes) > 5 {
		wb.writeTimes = wb.writeTimes[len(wb.writeTimes)-5:]
	}
}

func (wb *WriteBuffer) Shutdown() {
	// Set closed first so no new flush goroutines call wg.Add after wg.Wait returns.
	atomic.StoreInt32(&wb.closed, 1)
	close(wb.shutdownChan)
	wb.wg.Wait()
	wb.cancel()
}

func (wb *WriteBuffer) adjustBatchSize() {
	wb.mu.Lock()
	writeTimes := make([]time.Duration, len(wb.writeTimes))
	copy(writeTimes, wb.writeTimes)
	wb.mu.Unlock()

	if len(writeTimes) == 0 {
		return
	}

	var avgDuration time.Duration
	for _, t := range writeTimes {
		avgDuration += t
	}
	avgDuration /= time.Duration(len(writeTimes))

	currentSize := atomic.LoadInt32(&wb.batchSize)
	newSize := currentSize

	if avgDuration < 100*time.Millisecond && currentSize < int32(wb.maxBatchSize) {
		newSize = int32(float64(currentSize) * 1.1)
	} else if avgDuration > 500*time.Millisecond && currentSize > int32(wb.minBatchSize) {
		newSize = int32(float64(currentSize) * 0.9)
	}

	if newSize < int32(wb.minBatchSize) {
		newSize = int32(wb.minBatchSize)
	} else if newSize > int32(wb.maxBatchSize) {
		newSize = int32(wb.maxBatchSize)
	}

	if newSize != currentSize {
		atomic.StoreInt32(&wb.batchSize, newSize)
		slog.Debug("adjusted batch size", "old", currentSize, "new", newSize, "avgDuration", avgDuration)
	}
}
