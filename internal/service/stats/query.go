package stats

import (
	"context"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// QueryService 管理查询的创建、执行和缓存
type QueryService struct {
	runner *QueryRunner
}

// NewQueryService 创建新的查询服务
func NewQueryService(runner *QueryRunner) *QueryService {
	return &QueryService{
		runner: runner,
	}
}

// CreateQuery 创建新查询
func (qs *QueryService) CreateQuery(params *types.Params) (*types.Query, error) {
	// 验证参数
	if err := validateDate(params); err != nil {
		return nil, err
	}
	if err := validatePeriod(params); err != nil {
		return nil, err
	}
	if err := validateIntervals(params); err != nil {
		return nil, err
	}
	if err := validatePagination(params); err != nil {
		return nil, err
	}
	if err := validateDimensions(params); err != nil {
		return nil, err
	}

	if err := validateFilters(nil, params.Filters); err != nil {
		return nil, err
	}
	metrics, err := parseAndValidateMetrics(params)
	if err != nil {
		return nil, err
	}
	if err := params.ParsePeriodToUTCTimeRange(params.Timezone); err != nil {
		return nil, err
	}

	if err := ensureCustomPropsAccess(nil, params); err != nil {
		return nil, err
	}

	// 创建查询副本，避免修改原始参数
	query := &types.Query{
		SiteID:                 params.SiteID,
		UTCTimeRange:           params.UTCTimeRange,
		ComparisonUTCTimeRange: params.ComparisonUTCTimeRange,
		Interval:               params.Interval,
		Period:                 params.Period,
		Date:                   params.Date,
		From:                   params.From,
		To:                     params.To,
		Timezone:               params.Timezone,
		Dimensions:             params.Dimensions,
		Metrics:                metrics,
		Filters:                params.Filters,
		// TimeOnPageData:         params.TimeOnPageData,
		// SampleThreshold:        params.SampleThreshold,
		// Now:                    params.Now,
		// SiteNativeStatsStartAt: params.SiteNativeStatsStartAt,
		Pagination: params.Pagination,
		OrderBy:    params.OrderBy,
	}

	// 设置默认值
	if query.Now.IsZero() {
		query.Now = time.Now()
	}
	if query.SampleThreshold == 0 {
		query.SampleThreshold = 10000 // 默认采样阈值
	}

	return query, nil
}

// ExecuteQuery 执行查询
func (qs *QueryService) Execute(ctx context.Context, q *types.Query, site *types.Site) (*types.QueryResult, error) {
	// 执行查询
	result, err := qs.runner.RunQuery(ctx, q, site)
	if err != nil {
		return nil, err
	}

	return result, nil
}
