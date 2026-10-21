package session

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
	batchSize     int
	flushInterval time.Duration
	closed        int32 // atomic; Shutdown 置 1，阻止新的 flush goroutine 在 wg.Wait 后再 wg.Add

	buffer []*models.Sessions
	// inflight 记录已从 buffer 取出、正在写入 ClickHouse（含重试）但尚未确认落库的批次。
	// FindPending 需要同时扫描 buffer 与 inflight，否则 flush 窗口内的行会"既不在内存也不在 CH"。
	inflight map[*[]*models.Sessions]struct{}

	flushSem chan struct{} // 限制并发 flush goroutine 数量，防止下游阻塞时无界堆积

	shutdownChan chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc

	batchPool sync.Pool
}

// maxConcurrentFlushes 限制同时进行的 flush goroutine 数量。
const maxConcurrentFlushes = 8

func NewWriteBuffer(ctx context.Context, batchSize int, flushInterval time.Duration) *WriteBuffer {
	ctx, cancel := context.WithCancel(ctx)

	wb := &WriteBuffer{
		batchSize:     batchSize,
		flushInterval: flushInterval,
		flushSem:      make(chan struct{}, maxConcurrentFlushes),
		buffer:        make([]*models.Sessions, 0, batchSize),
		inflight:      make(map[*[]*models.Sessions]struct{}),
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
	// Shutdown 后拒绝新 session，避免 wg.Add 与 wg.Wait 竞态导致 panic
	if atomic.LoadInt32(&wb.closed) == 1 {
		return
	}

	wb.mu.Lock()
	wb.buffer = append(wb.buffer, session)
	shouldFlush := len(wb.buffer) >= wb.batchSize
	wb.mu.Unlock()

	slog.Debug("add session to buffer", "session", session)
	if shouldFlush {
		// 独立 goroutine 写入，不阻塞调用方（session manager 的 balancer worker）；
		// 并发数受 flushSem 限制，达到上限则跳过，交由 flushWorker 定时器兜底。
		select {
		case wb.flushSem <- struct{}{}:
			wb.wg.Add(1)
			go func() {
				defer wb.wg.Done()
				defer func() { <-wb.flushSem }()
				wb.flushAndWrite()
			}()
		default:
		}
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
	wb.inflight[batch] = struct{}{}
	wb.mu.Unlock()

	if err := wb.writeBatch(*batch); err != nil {
		slog.Error("failed to write session batch", "error", err)
	}

	wb.mu.Lock()
	delete(wb.inflight, batch)
	wb.mu.Unlock()
	*batch = (*batch)[:0]
	wb.batchPool.Put(batch)
}

// FindPending 在尚未确认落库的行（buffer + inflight 批次）中查找 userId/siteId 的最新状态行：
// Sign=+1、timestamp 不早于 notBefore、version 最大者。返回的指针指向 buffer 内部对象，调用方不得修改。
// 复杂度 O(pending rows)，仅在 session 缓存 miss 时调用。
func (wb *WriteBuffer) FindPending(userId, siteId uint64, notBefore time.Time) *models.Sessions {
	var best *models.Sessions
	consider := func(s *models.Sessions) {
		if s == nil || s.Sign != 1 || s.UserId != userId || s.SiteId != siteId || s.Timestamp.Before(notBefore) {
			return
		}
		if best == nil || s.Version > best.Version {
			best = s
		}
	}

	wb.mu.Lock()
	defer wb.mu.Unlock()
	for _, s := range wb.buffer {
		consider(s)
	}
	for batch := range wb.inflight {
		for _, s := range *batch {
			consider(s)
		}
	}
	return best
}

// FindPendingAt finds the newest pending state whose activity window contains
// the event. Stale versions must not make an expired session active again.
func (wb *WriteBuffer) FindPendingAt(userID, siteID uint64, timestamp time.Time) *models.Sessions {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	latest := make(map[uint64]*models.Sessions)
	consider := func(s *models.Sessions) {
		if s == nil || s.Sign != 1 || s.UserId != userID || s.SiteId != siteID {
			return
		}
		if old := latest[s.SessionId]; old == nil || s.Version > old.Version {
			latest[s.SessionId] = s
		}
	}
	for _, s := range wb.buffer {
		consider(s)
	}
	for batch := range wb.inflight {
		for _, s := range *batch {
			consider(s)
		}
	}
	var best *models.Sessions
	for _, s := range latest {
		if sessionContainsEvent(s, timestamp) && (best == nil || s.Start.After(best.Start) || (s.Start.Equal(best.Start) && s.Version > best.Version)) {
			best = s
		}
	}
	return best
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

	// 一次批次生成一个去重 token，所有重试复用它，避免重试重复插入 ±1 会话行破坏 collapse 计数。
	insertCtx := repository.WithInsertDedupToken(wb.ctx)

	for i := range retries {
		if err = repo.BatchInsert(insertCtx, batch); err == nil {
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
	// 先置 closed，确保 wg.Wait 之后不再有新的 flush goroutine 调用 wg.Add
	atomic.StoreInt32(&wb.closed, 1)
	close(wb.shutdownChan)
	wb.wg.Wait()
	wb.cancel()
}
