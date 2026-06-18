package stats

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

// SuggestionItem 建议项
type SuggestionItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SuggestionService 筛选器建议服务，提供自定义属性 key/value 的自动补全。
type SuggestionService struct {
	conn driver.Conn
}

// NewSuggestionService 创建建议服务实例。
func NewSuggestionService() *SuggestionService {
	return &SuggestionService{conn: cl.GetConnection()}
}

// GetPropKeys 获取站点在指定时间范围内的所有自定义属性键名。
// 按出现频率降序排列，最多返回 25 条。
func (s *SuggestionService) GetPropKeys(ctx context.Context, siteID string, startTime, endTime string, query string) ([]SuggestionItem, error) {
	filterQuery := "%"
	if query != "" {
		filterQuery = "%" + query + "%"
	}

	sql := `
		SELECT meta_key, count(*) AS cnt
		FROM zenstats_events_db.events
		ARRAY JOIN meta.key AS meta_key
		WHERE site_id = ?
		  AND timestamp >= ?
		  AND timestamp <= ?
		  AND meta_key ILIKE ?
		GROUP BY meta_key
		ORDER BY cnt DESC
		LIMIT 25
	`

	rows, err := s.conn.Query(ctx, sql, siteID, startTime, endTime, filterQuery)
	if err != nil {
		return nil, fmt.Errorf("query prop keys: %w", err)
	}
	defer rows.Close()

	var items []SuggestionItem
	for rows.Next() {
		var value string
		var cnt uint64
		if err := rows.Scan(&value, &cnt); err != nil {
			continue
		}
		items = append(items, SuggestionItem{Value: value, Label: value})
	}
	if items == nil {
		items = []SuggestionItem{}
	}
	return items, nil
}

// GetPropValues 获取站点在指定时间范围内，指定属性的可选值列表。
// 按出现频率降序排列，最多返回 25 条。支持模糊搜索。
func (s *SuggestionService) GetPropValues(ctx context.Context, siteID string, startTime, endTime string, propKey, query string) ([]SuggestionItem, error) {
	filterQuery := "%"
	if query != "" {
		filterQuery = "%" + query + "%"
	}

	// 使用数组位置索引获取指定 key 对应的 value
	sql := `
		SELECT prop_val, count(*) AS cnt
		FROM (
			SELECT meta.value[indexOf(meta.key, ?)] AS prop_val
			FROM zenstats_events_db.events
			WHERE site_id = ?
			  AND timestamp >= ?
			  AND timestamp <= ?
			  AND has(meta.key, ?)
		)
		WHERE prop_val ILIKE ?
		GROUP BY prop_val
		ORDER BY cnt DESC
		LIMIT 25
	`

	rows, err := s.conn.Query(ctx, sql, propKey, siteID, startTime, endTime, propKey, filterQuery)
	if err != nil {
		return nil, fmt.Errorf("query prop values: %w", err)
	}
	defer rows.Close()

	var items []SuggestionItem
	for rows.Next() {
		var value string
		var cnt uint64
		if err := rows.Scan(&value, &cnt); err != nil {
			continue
		}
		items = append(items, SuggestionItem{Value: value, Label: value})
	}
	if items == nil {
		items = []SuggestionItem{}
	}

	// 如果查询词可能匹配空值，追加 (none) 选项
	if query == "" || strings.Contains(strings.ToLower("(none)"), strings.ToLower(query)) {
		hasNone, _ := s.hasEmptyPropValues(ctx, siteID, startTime, endTime, propKey)
		if hasNone {
			items = append(items, SuggestionItem{Value: "(none)", Label: "(none)"})
		}
	}

	return items, nil
}

// hasEmptyPropValues 检查是否存在无该属性的记录。
func (s *SuggestionService) hasEmptyPropValues(ctx context.Context, siteID, startTime, endTime, propKey string) (bool, error) {
	sql := `
		SELECT count(*) > 0
		FROM zenstats_events_db.events
		WHERE site_id = ?
		  AND timestamp >= ?
		  AND timestamp <= ?
		  AND NOT has(meta.key, ?)
		LIMIT 1
	`

	row := s.conn.QueryRow(ctx, sql, siteID, startTime, endTime, propKey)
	var hasNone bool
	if err := row.Scan(&hasNone); err != nil {
		return false, nil
	}
	return hasNone, nil
}
