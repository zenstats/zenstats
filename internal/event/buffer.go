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
	mu            sync.RWMutex
	wg            sync.WaitGroup
	batchSize     int32          // 改为原子操作安全的int32类型
	flushInterval time.Duration
	minBatchSize  int
	maxBatchSize  int
	eventCounter  int64
	writeTimes    []time.Duration // 记录最近写入时间，用于动态调整

	buffer    []*models.Events
	flushChan chan []*models.Events

	shutdownChan chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc

	batchPool sync.Pool
}

func NewWriteBuffer(ctx context.Context, batchSize int, flushInterval time.Duration) *WriteBuffer {
	ctx, cancel := context.WithCancel(ctx)

	// 设置批处理大小范围 (默认50-500)
	minBatchSize := batchSize / 2
	if minBatchSize < 50 {
		minBatchSize = 50
	}
	maxBatchSize := batchSize * 2
	if maxBatchSize > 500 {
		maxBatchSize = 500
	}

	wb := &WriteBuffer{
		batchSize:     int32(batchSize),
		minBatchSize:  minBatchSize,
		maxBatchSize:  maxBatchSize,
		flushInterval: flushInterval,
		buffer:        make([]*models.Events, 0, batchSize),
		flushChan:     make(chan []*models.Events, 10),
		shutdownChan:  make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
		writeTimes:    make([]time.Duration, 0, 5), // 最多记录5次写入时间
	}
	wb.batchPool.New = func() any {
		s := make([]*models.Events, 0, int(wb.batchSize))
		return &s
	}

	return wb
}

func (wb *WriteBuffer) Add(event *models.Events) {
	wb.mu.Lock()
	wb.buffer = append(wb.buffer, event)
	shouldFlush := len(wb.buffer) >= int(wb.batchSize)
	wb.mu.Unlock()
	// 动态调整批处理大小 (每1000个事件调整一次)
	if atomic.AddInt64(&wb.eventCounter, 1) % 1000 == 0 {
		go wb.adjustBatchSize()
	}
	slog.Debug("add event to buffer", "event", event)
	if shouldFlush {
		wb.flush()
	}
}

func (wb *WriteBuffer) flush() {
	wb.mu.Lock()
	if len(wb.buffer) == 0 {
		wb.mu.Unlock()
		return
	}

	batch := wb.batchPool.Get().(*[]*models.Events)
	*batch = append(*batch, wb.buffer...)
	wb.buffer = wb.buffer[:0]

	wb.mu.Unlock()

	select {
	case wb.flushChan <- *batch:
	case <-wb.shutdownChan:
	case <-wb.ctx.Done():
	}
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
			wb.flush()

		case batch := <-wb.flushChan:
			if err := wb.writeBatch(batch); err != nil {
				slog.Error("failed to write batch after retries", "error", err)
			}
			batch = batch[:0]
			wb.batchPool.Put(&batch)

		case <-wb.shutdownChan:
			wb.drainBuffer()
			return

		case <-wb.ctx.Done():
			wb.drainBuffer()
			return
		}
	}
}

func (wb *WriteBuffer) writeBatch(batch []*models.Events) error {
	retries := 5
	var err error
	backoff := []time.Duration{100*time.Millisecond, 300*time.Millisecond, 1*time.Second, 3*time.Second, 5*time.Second}

	// 记录写入时间用于动态调整批处理大小
	startTime := time.Now()
	for i := 0; i < retries; i++ {
		if err = repository.GetEventsRepository().BatchInsert(wb.ctx, batch); err == nil {
			duration := time.Since(startTime)
			wb.recordWriteTime(duration)
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

// 记录写入时间，最多保留最近5次
func (wb *WriteBuffer) recordWriteTime(duration time.Duration) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.writeTimes = append(wb.writeTimes, duration)
	// 只保留最近5次写入时间
	if len(wb.writeTimes) > 5 {
		wb.writeTimes = wb.writeTimes[len(wb.writeTimes)-5:]
	}
}

func (wb *WriteBuffer) drainBuffer() {
	wb.mu.Lock()
	if len(wb.buffer) > 0 {
		batchPtr := wb.batchPool.Get().(*[]*models.Events)
		*batchPtr = append(*batchPtr, wb.buffer...)
		wb.buffer = wb.buffer[:0]

		select {
		case wb.flushChan <- *batchPtr:
		default:
			// 如果通道已满，直接处理并归还
			_ = wb.writeBatch(*batchPtr)
			*batchPtr = (*batchPtr)[:0]
			wb.batchPool.Put(batchPtr)
		}

	}
	wb.mu.Unlock()

	close(wb.flushChan)
	for batch := range wb.flushChan {
		_ = wb.writeBatch(batch)

		batch = batch[:0]
		wb.batchPool.Put(&batch)
	}
}

func (wb *WriteBuffer) Shutdown() {
	close(wb.shutdownChan)
	wb.wg.Wait()
	wb.cancel()
}

// adjustBatchSize 根据最近写入性能动态调整批处理大小
func (wb *WriteBuffer) adjustBatchSize() {
	wb.mu.RLock()
	writeTimes := make([]time.Duration, len(wb.writeTimes))
	copy(writeTimes, wb.writeTimes)
	wb.mu.RUnlock()

	if len(writeTimes) == 0 {
		return
	}

	// 计算平均写入时间
	avgDuration := time.Duration(0)
	for _, t := range writeTimes {
		avgDuration += t
	}
	avgDuration /= time.Duration(len(writeTimes))

	currentSize := atomic.LoadInt32(&wb.batchSize)
	newSize := currentSize

	// 根据平均写入时间调整批处理大小
	// 写入快则增加批处理大小，写入慢则减小
	if avgDuration < 100*time.Millisecond && currentSize < int32(wb.maxBatchSize) {
		newSize = int32(float64(currentSize) * 1.1) // 增加10%
	} else if avgDuration > 500*time.Millisecond && currentSize > int32(wb.minBatchSize) {
		newSize = int32(float64(currentSize) * 0.9) // 减少10%
	}

	// 限制在min和max之间
	if newSize < int32(wb.minBatchSize) {
		newSize = int32(wb.minBatchSize)
	} else if newSize > int32(wb.maxBatchSize) {
		newSize = int32(wb.maxBatchSize)
	}

	if newSize != currentSize {
		atomic.StoreInt32(&wb.batchSize, newSize)
		slog.Debug("Adjusted batch size", "old", currentSize, "new", newSize, "avgDuration", avgDuration)
	}
}
