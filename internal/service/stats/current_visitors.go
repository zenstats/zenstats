package stats

import (
	"context"
	"fmt"
	"time"
)

// CurrentVisitors 实时访客统计结果
type CurrentVisitors struct {
	Total       int       `json:"total"`
	Visitors    int       `json:"visitors"`
	Sessions    int       `json:"sessions"`
	LastUpdated time.Time `json:"last_updated"`
}

// CurrentVisitorsService 处理实时访客统计
type CurrentVisitorsService struct {
	runner *QueryRunner
}

// NewCurrentVisitorsService 创建新的实时访客服务
func NewCurrentVisitorsService(runner *QueryRunner) *CurrentVisitorsService {
	return &CurrentVisitorsService{
		runner: runner,
	}
}

// GetCurrentVisitors 获取当前访客统计
func (s *CurrentVisitorsService) GetCurrentVisitors(ctx context.Context, siteID string, since time.Duration) (*CurrentVisitors, error) {
	// 计算时间范围（默认最近5分钟）
	if since == 0 {
		since = 5 * time.Minute
	}

	startTime := time.Now().Add(-since)

	// 构建查询
	sql := `
		SELECT
			count(distinct user_id) as visitors,
			count(distinct session_id) as sessions
		FROM events
		WHERE
			site_id = ?
			AND name != "engagement"
			AND timestamp >= ?
	`

	// 执行查询
	rows, err := s.runner.conn.Query(ctx, sql, siteID, startTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query current visitors: %v", err)
	}
	defer rows.Close()

	// 处理结果
	var visitors, sessions int
	found := false

	for rows.Next() {
		if err := rows.Scan(&visitors, &sessions); err != nil {
			return nil, err
		}
		found = true
	}

	if !found {
		return &CurrentVisitors{
				Total:       0,
				Visitors:    0,
				Sessions:    0,
				LastUpdated: time.Now(),
			},
			nil
	}

	return &CurrentVisitors{
			Total:       visitors,
			Visitors:    visitors,
			Sessions:    sessions,
			LastUpdated: time.Now(),
		},
		nil
}

// GetCurrentVisitorsByPage 获取按页面分组的当前访客
func (s *CurrentVisitorsService) GetCurrentVisitorsByPage(ctx context.Context, siteID string, since time.Duration, limit int) ([]map[string]any, error) {
	// 计算时间范围
	if since == 0 {
		since = 5 * time.Minute
	}

	startTime := time.Now().Add(-since)

	// 构建查询
	sql := `
		SELECT
			page,
			count(distinct user_id) as visitors,
			count(distinct session_id) as sessions
		FROM events
		WHERE
			site_id = ?
			AND name != "engagement"
			AND timestamp >= ?
			AND page != ''
		GROUP BY page
		ORDER BY visitors DESC
		LIMIT ?
	`

	// 执行查询
	rows, err := s.runner.conn.Query(ctx, sql, siteID, startTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query current visitors by page: %v", err)
	}
	defer rows.Close()

	// 处理结果
	results := []map[string]any{}

	for rows.Next() {
		var page string
		var visitors, sessions int

		if err := rows.Scan(&page, &visitors, &sessions); err != nil {
			return nil, err
		}

		results = append(results, map[string]any{
			"page":     page,
			"visitors": visitors,
			"sessions": sessions,
		})
	}

	return results, nil
}
