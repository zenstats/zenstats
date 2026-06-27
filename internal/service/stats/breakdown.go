package stats

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// BreakdownResult 细分结果
type BreakdownResult struct {
	Dimension string           `json:"dimension"`
	Values    []BreakdownValue `json:"values"`
	Total     map[string]any   `json:"total,omitempty"`
}

// BreakdownValue 单个细分值
type BreakdownValue struct {
	Value any            `json:"value"`
	Stats map[string]any `json:"stats"`
}

// BreakdownService 处理数据细分
type BreakdownService struct {
	qs *QueryService
}

// NewBreakdownService 创建新的细分服务
func NewBreakdownService(qs *QueryService) *BreakdownService {
	return &BreakdownService{
		qs: qs,
	}
}

// GetBreakdown 按指定维度获取细分数据
func (bs *BreakdownService) GetBreakdown(ctx context.Context, params *types.Query, dimension string, limit int) (*BreakdownResult, error) {
	// 创建带维度的查询参数
	breakdownParams := &types.Params{
		SiteID:       params.SiteID,
		UTCTimeRange: params.UTCTimeRange,
		Metrics:      params.Metrics,
		Dimensions:   []string{dimension},
		Filters:      params.Filters,
		Interval:     "", // 细分查询不需要时间间隔
	}

	query, err := bs.qs.CreateQuery(breakdownParams)
	if err != nil {
		return nil, err
	}
	site := &types.Site{ID: query.SiteID, UserID: query.UserID, Timezone: query.Timezone}
	// 执行查询
	result, err := bs.qs.runner.RunQuery(ctx, query, site)
	if err != nil {
		return nil, fmt.Errorf("failed to run breakdown query: %v", err)
	}

	// 处理结果
	breakdownValues, total, err := bs.processBreakdownResult(result, dimension, limit)
	if err != nil {
		return nil, err
	}

	return &BreakdownResult{
			Dimension: dimension,
			Values:    breakdownValues,
			Total:     total,
		},
		nil
}

// GetMultiBreakdown 获取多维度组合细分数据
func (bs *BreakdownService) GetMultiBreakdown(ctx context.Context, params *types.Query, dimensions []string, limit int) ([]*BreakdownResult, error) {
	if len(dimensions) == 0 {
		return nil, fmt.Errorf("at least one dimension is required")
	}

	results := make([]*BreakdownResult, len(dimensions))

	// 为每个维度获取细分数据
	for i, dimension := range dimensions {
		breakdown, err := bs.GetBreakdown(ctx, params, dimension, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get breakdown for dimension %s: %v", dimension, err)
		}
		results[i] = breakdown
	}

	return results, nil
}

// processBreakdownResult 处理细分查询结果
func (bs *BreakdownService) processBreakdownResult(result *types.QueryResult, dimension string, limit int) ([]BreakdownValue, map[string]any, error) {
	// 提取维度列名
	dimensionCol := getDimensionColumnName(dimension)

	// 查找维度列索引
	dimIndex := -1
	for i, col := range result.Columns {
		if col == dimensionCol {
			dimIndex = i
			break
		}
	}

	if dimIndex == -1 {
		return nil, nil, fmt.Errorf("dimension column %s not found in results", dimensionCol)
	}

	// 聚合结果
	valueMap := make(map[any]map[string]any)
	metrics := make([]string, 0)

	// 初始化指标列表
	for _, col := range result.Columns {
		if col != dimensionCol {
			metrics = append(metrics, col)
		}
	}

	// 处理每一行数据
	for _, row := range result.Data {
		// 获取维度值
		val := row[dimensionCol]

		// 初始化指标值
		if _, exists := valueMap[val]; !exists {
			valueMap[val] = make(map[string]any)
		}

		// 聚合指标
		for _, metric := range metrics {
			valueMap[val][metric] = row[metric]
		}
	}

	// 转换为切片并排序
	values := make([]BreakdownValue, 0, len(valueMap))
	for val, stats := range valueMap {
		values = append(values, BreakdownValue{
			Value: val,
			Stats: stats,
		})
	}

	// 按主要指标排序
	if len(metrics) > 0 {
		sort.Slice(values, func(i, j int) bool {
			// 假设第一个指标为主要排序指标
			metric := metrics[0]

			// 比较数值
			valI, okI := values[i].Stats[metric].(float64)
			valJ, okJ := values[j].Stats[metric].(float64)

			if okI && okJ {
				return valI > valJ // 降序排列
			}

			return false
		})
	}

	// 计算总计
	total := calculateTotal(values, metrics)

	// 应用限制
	if limit > 0 && len(values) > limit {
		values = values[:limit]
	}

	return values, total, nil
}

// getDimensionColumnName 获取维度对应的列名。
// processResults 将单维度查询的列重命名为 "name"，因此命名空间维度（如 visit:country）
// 在结果集中始终以 "name" 作为列名返回。
func getDimensionColumnName(dimension string) string {
	switch dimension {
	case "date":
		return "date"
	case "hour":
		return "hour"
	case "country":
		return "country"
	case "device":
		return "device"
	default:
		// 命名空间维度（visit:xxx / event:xxx）经 processResults 重命名为 "name"
		return "name"
	}
}

// calculateTotal 计算所有细分值的总计
func calculateTotal(values []BreakdownValue, metrics []string) map[string]any {
	total := make(map[string]any)

	for _, metric := range metrics {
		var sum float64
		count := 0

		for _, value := range values {
			if val, ok := value.Stats[metric].(float64); ok {
				sum += val
				count++
			}
		}

		if strings.HasSuffix(metric, "_rate") {
			// Weighted average requires per-row visitor counts; set nil to signal unavailability.
			total[metric] = nil
		} else if count > 0 {
			total[metric] = sum
		} else {
			total[metric] = 0
		}
	}

	return total
}
