package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
)

// advisoryLockKey maps an arbitrary job name to a stable 64-bit advisory lock key.
func advisoryLockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64())
}

// sqlDB returns the shared connection pool (nil if unavailable).
func (c *Client) sqlDB() *sql.DB {
	if c == nil {
		return nil
	}
	return c.DB
}

// TryAdvisoryLock 尝试获取以 name 命名的 PostgreSQL 会话级咨询锁（pg_try_advisory_lock）。
//
// 锁绑定在一条专用连接上，直到调用返回的 release 函数才释放；多副本部署时用它保证
// 定时任务（邮件报告 / 流量告警 / GeoIP 更新）在同一时刻只由一个实例执行。
// 返回 acquired=false 表示锁已被其他实例持有，调用方应直接跳过本轮。
func (c *Client) TryAdvisoryLock(ctx context.Context, name string) (release func(), acquired bool, err error) {
	db := c.sqlDB()
	if db == nil {
		return nil, false, fmt.Errorf("postgresql: sql.DB unavailable for advisory lock")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("postgresql: acquire connection: %w", err)
	}
	key := advisoryLockKey(name)
	var got bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, false, fmt.Errorf("postgresql: pg_try_advisory_lock: %w", err)
	}
	if !got {
		_ = conn.Close()
		return nil, false, nil
	}
	release = func() {
		// 使用独立的 context：即便任务 ctx 已取消，也要尽力释放锁；连接关闭本身也会释放会话锁。
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
		_ = conn.Close()
	}
	return release, true, nil
}
