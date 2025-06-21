package event

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
	"github.com/zenstats/zenstats/internal/store/clickhouse/repository"
)

type WriteBuffer struct {
	mu            sync.Mutex
	wg            sync.WaitGroup
	batchSize     int
	flushInterval time.Duration

	buffer    []*models.Events
	flushChan chan []*models.Events

	shutdownChan chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc

	batchPool sync.Pool
}

func NewWriteBuffer(ctx context.Context, batchSize int, flushInterval time.Duration) *WriteBuffer {
	ctx, cancel := context.WithCancel(ctx)

	wb := &WriteBuffer{
		batchSize:     batchSize,
		flushInterval: flushInterval,
		buffer:        make([]*models.Events, 0, batchSize),
		flushChan:     make(chan []*models.Events, 10),
		shutdownChan:  make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}
	wb.batchPool.New = func() any {
		s := make([]*models.Events, 0, wb.batchSize)
		return &s
	}

	return wb
}

func (wb *WriteBuffer) Add(event *models.Events) {
	wb.mu.Lock()
	wb.buffer = append(wb.buffer, event)
	shouldFlush := len(wb.buffer) >= wb.batchSize
	wb.mu.Unlock()
	slog.Debug("add event to buffer", "event", event)
	if shouldFlush {
		wb.flush()
	}
}

func (wb *WriteBuffer) flush() {
	slog.Debug("flush buffer")
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
	retries := 3
	var err error

	for i := range retries {
		if err = repository.GetEventsRepository().BatchInsert(wb.ctx, batch); err == nil {
			return nil
		}
		slog.Debug("failed to flush batch after retries")
		select {
		case <-time.After(time.Second * time.Duration(i+1)):
		case <-wb.ctx.Done():
			return wb.ctx.Err()
		}
	}
	return err
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
