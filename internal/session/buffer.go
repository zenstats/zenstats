package session

import (
	"context"
	"fmt"
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

	buffer []*models.Sessions

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
		buffer:        make([]*models.Sessions, 0, batchSize),
		shutdownChan:  make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}
	wb.batchPool.New = func() any {
		s := make([]*models.Sessions, 0, wb.batchSize)
		return &s
	}

	return wb
}

func (wb *WriteBuffer) Add(session *models.Sessions) {
	wb.mu.Lock()
	wb.buffer = append(wb.buffer, session)
	shouldFlush := len(wb.buffer) >= wb.batchSize
	wb.mu.Unlock()

	slog.Debug("add session to buffer", "session", session)
	if shouldFlush {
		// 独立 goroutine 写入，不阻塞调用方（session manager 的 balancer worker）
		wb.wg.Add(1)
		go func() {
			defer wb.wg.Done()
			wb.flushAndWrite()
		}()
	}
}

// flushAndWrite 从 buffer 取出数据并直接写入 ClickHouse，无需 channel 中转。
// 并发调用安全：只有第一个能取到数据，其余见空 buffer 立即返回。
func (wb *WriteBuffer) flushAndWrite() {
	wb.mu.Lock()
	if len(wb.buffer) == 0 {
		wb.mu.Unlock()
		return
	}
	batch := wb.batchPool.Get().(*[]*models.Sessions)
	*batch = append(*batch, wb.buffer...)
	wb.buffer = wb.buffer[:0]
	wb.mu.Unlock()

	if err := wb.writeBatch(*batch); err != nil {
		slog.Error("failed to write session batch", "error", err)
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

func (wb *WriteBuffer) writeBatch(batch []*models.Sessions) error {
	repo := repository.GetSessionsRepository()
	if repo == nil {
		return fmt.Errorf("sessions repository not initialized")
	}

	retries := 3
	var err error

	for i := range retries {
		if err = repo.BatchInsert(wb.ctx, batch); err == nil {
			return nil
		}

		select {
		case <-time.After(time.Second * time.Duration(i+1)):
		case <-wb.ctx.Done():
			return wb.ctx.Err()
		}
	}
	return err
}

func (wb *WriteBuffer) Shutdown() {
	close(wb.shutdownChan)
	wb.wg.Wait()
	wb.cancel()
}
