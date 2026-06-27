package event

import (
	"context"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql/ent/monthlyeventcount"
	"github.com/zenstats/zenstats/pkg/globals"
)

// MonthlyQuota 内存计数器，按用户统计当月事件数
type MonthlyQuota struct {
	mu     sync.RWMutex
	counts map[int64]int64 // user_id -> event count
	year   int
	month  int
	stopCh chan struct{}
}

var (
	monthlyQuotaInstance *MonthlyQuota
	quotaOnce            sync.Once
)

// GetMonthlyQuota 获取 MonthlyQuota 单例
func GetMonthlyQuota() *MonthlyQuota {
	quotaOnce.Do(func() {
		now := time.Now()
		monthlyQuotaInstance = &MonthlyQuota{
			counts: make(map[int64]int64),
			year:   now.Year(),
			month:  int(now.Month()),
			stopCh: make(chan struct{}),
		}
		monthlyQuotaInstance.loadFromDB(context.Background())
		go monthlyQuotaInstance.startFlusher()
	})
	return monthlyQuotaInstance
}

// Increment 增加用户当月事件计数，返回新值
func (q *MonthlyQuota) Increment(userID int64) int64 {
	q.mu.Lock()
	oldCounts, oldYear, oldMonth, rotated := q.checkAndRotate()
	q.counts[userID]++
	count := q.counts[userID]
	q.mu.Unlock()

	// flushSnapshot 在锁外异步执行，不阻塞 Increment
	if rotated {
		go q.flushSnapshot(oldCounts, oldYear, oldMonth)
	}
	return count
}

// Get 获取用户当月事件计数
func (q *MonthlyQuota) Get(userID int64) int64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.counts[userID]
}

// checkAndRotate 检测是否跨月并重置状态，必须在 q.mu.Lock() 持有期间调用。
// 若跨月，返回旧月份数据供调用方在锁外持久化。
func (q *MonthlyQuota) checkAndRotate() (oldCounts map[int64]int64, oldYear, oldMonth int, rotated bool) {
	now := time.Now()
	if now.Year() == q.year && int(now.Month()) == q.month {
		return nil, 0, 0, false
	}
	oldCounts = q.counts
	oldYear, oldMonth = q.year, q.month
	q.counts = make(map[int64]int64)
	q.year = now.Year()
	q.month = int(now.Month())
	return oldCounts, oldYear, oldMonth, true
}

// loadFromDB 从数据库加载当月数据
func (q *MonthlyQuota) loadFromDB(ctx context.Context) {
	db := globals.GetDB()
	if db == nil {
		return
	}

	counts, err := db.Client.MonthlyEventCount.Query().
		Where(
			monthlyeventcount.Year(q.year),
			monthlyeventcount.Month(q.month),
		).
		All(ctx)
	if err != nil {
		return
	}

	for _, c := range counts {
		q.counts[c.UserID] = c.Count
	}
}

// flushToDB 将当前内存计数快照持久化到数据库（在锁外调用）
func (q *MonthlyQuota) flushToDB() {
	q.mu.RLock()
	snapshot := make(map[int64]int64, len(q.counts))
	for k, v := range q.counts {
		snapshot[k] = v
	}
	year, month := q.year, q.month
	q.mu.RUnlock()

	q.flushSnapshot(snapshot, year, month)
}

// flushSnapshot 将给定快照写入数据库，不持有任何锁
func (q *MonthlyQuota) flushSnapshot(snapshot map[int64]int64, year, month int) {
	db := globals.GetDB()
	if db == nil {
		return
	}

	ctx := context.Background()
	for userID, count := range snapshot {
		existing, _ := db.Client.MonthlyEventCount.Query().
			Where(
				monthlyeventcount.UserID(userID),
				monthlyeventcount.Year(year),
				monthlyeventcount.Month(month),
			).
			Only(ctx)

		if existing != nil {
			_, _ = db.Client.MonthlyEventCount.UpdateOne(existing).
				SetCount(count).
				Save(ctx)
		} else {
			_, _ = db.Client.MonthlyEventCount.Create().
				SetUserID(userID).
				SetYear(year).
				SetMonth(month).
				SetCount(count).
				Save(ctx)
		}
	}
}

// startFlusher 启动定时持久化协程（每小时执行一次）
func (q *MonthlyQuota) startFlusher() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			q.mu.Lock()
			oldCounts, oldYear, oldMonth, rotated := q.checkAndRotate()
			q.mu.Unlock()
			if rotated {
				q.flushSnapshot(oldCounts, oldYear, oldMonth)
			}
			q.flushToDB()
		case <-q.stopCh:
			return
		}
	}
}

// Stop shuts down the background flusher goroutine.
func (q *MonthlyQuota) Stop() {
	close(q.stopCh)
}

// Flush 手动触发持久化（用于优雅关闭）
func (q *MonthlyQuota) Flush() {
	q.mu.Lock()
	oldCounts, oldYear, oldMonth, rotated := q.checkAndRotate()
	q.mu.Unlock()

	if rotated {
		q.flushSnapshot(oldCounts, oldYear, oldMonth)
	}
	q.flushToDB()
}
