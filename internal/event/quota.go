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
		}
		// 从数据库加载当月数据
		monthlyQuotaInstance.loadFromDB(context.Background())
		// 启动定时持久化（每小时）
		go monthlyQuotaInstance.startFlusher()
	})
	return monthlyQuotaInstance
}

// Increment 增加用户当月事件计数，返回新值
func (q *MonthlyQuota) Increment(userID int64) int64 {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.rotateIfNewMonth()

	q.counts[userID]++
	return q.counts[userID]
}

// Get 获取用户当月事件计数
func (q *MonthlyQuota) Get(userID int64) int64 {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.counts[userID]
}

// rotateIfNewMonth 检查是否跨月，跨月则归零
func (q *MonthlyQuota) rotateIfNewMonth() {
	now := time.Now()
	if now.Year() != q.year || int(now.Month()) != q.month {
		// 先持久化旧月份数据
		q.flushToDB()
		// 归零
		q.counts = make(map[int64]int64)
		q.year = now.Year()
		q.month = int(now.Month())
	}
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

// flushToDB 将内存计数持久化到数据库
func (q *MonthlyQuota) flushToDB() {
	db := globals.GetDB()
	if db == nil {
		return
	}

	ctx := context.Background()

	q.mu.RLock()
	// 复制数据以减少锁持有时间
	snapshot := make(map[int64]int64, len(q.counts))
	for k, v := range q.counts {
		snapshot[k] = v
	}
	year := q.year
	month := q.month
	q.mu.RUnlock()

	for userID, count := range snapshot {
		// 查找是否已有记录
		existing, _ := db.Client.MonthlyEventCount.Query().
			Where(
				monthlyeventcount.UserID(userID),
				monthlyeventcount.Year(year),
				monthlyeventcount.Month(month),
			).
			Only(ctx)

		if existing != nil {
			// 更新已有记录
			_, _ = db.Client.MonthlyEventCount.UpdateOne(existing).
				SetCount(count).
				Save(ctx)
		} else {
			// 创建新记录
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

	for range ticker.C {
		q.mu.Lock()
		q.rotateIfNewMonth()
		q.mu.Unlock()

		q.flushToDB()
	}
}

// Flush 手动触发持久化（用于优雅关闭）
func (q *MonthlyQuota) Flush() {
	q.mu.Lock()
	q.rotateIfNewMonth()
	q.mu.Unlock()

	q.flushToDB()
}
