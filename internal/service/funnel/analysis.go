package funnel

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

// AnalysisService 漏斗分析服务，基于 ClickHouse 查询漏斗转化数据。
type AnalysisService struct {
	conn driver.Conn
}

// NewAnalysisService 创建漏斗分析服务实例。
func NewAnalysisService() *AnalysisService {
	return &AnalysisService{
		conn: cl.GetConnection(),
	}
}

// FunnelStepResult 漏斗步骤分析结果。
type FunnelStepResult struct {
	StepOrder      int     `json:"step_order"`
	GoalName       string  `json:"goal_name"`
	Visitors       uint64  `json:"visitors"`
	DropOff        int64   `json:"drop_off"`
	ConversionRate float64 `json:"conversion_rate"`
}

// FunnelAnalysisResult 漏斗分析结果。
type FunnelAnalysisResult struct {
	TotalVisitors  uint64              `json:"total_visitors"`
	Steps          []*FunnelStepResult `json:"steps"`
	ConversionRate float64             `json:"conversion_rate"`
}

// FunnelStep 漏斗步骤定义。
type FunnelStep struct {
	GoalID      int64
	GoalType    string // "event" or "page"
	GoalValue   string // event name or page path
	GoalName    string
	StepOrder   int
}

// AnalysisRequest 漏斗分析请求。
type AnalysisRequest struct {
	SiteID    string
	Steps     []*FunnelStep
	StartTime time.Time
	EndTime   time.Time
}

// Analyze 执行漏斗分析。
func (s *AnalysisService) Analyze(ctx context.Context, req *AnalysisRequest) (*FunnelAnalysisResult, error) {
	if len(req.Steps) < 2 {
		return nil, fmt.Errorf("funnel must have at least 2 steps")
	}
	if len(req.Steps) > 8 {
		return nil, fmt.Errorf("funnel must have at most 8 steps")
	}

	// 构建漏斗分析查询
	// 使用窗口函数计算每个步骤的转化
	query, args := s.buildFunnelQuery(req)

	var totalVisitors uint64
	stepResults := make([]*FunnelStepResult, len(req.Steps))

	// 执行查询
	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute funnel query: %w", err)
	}
	defer rows.Close()

	// 处理结果
	stepIndex := 0
	for rows.Next() {
		var visitors uint64
		var stepOrder int

		if err := rows.Scan(&stepOrder, &visitors); err != nil {
			return nil, fmt.Errorf("failed to scan funnel result: %w", err)
		}

		if stepIndex == 0 {
			totalVisitors = visitors
		}

		step := req.Steps[stepOrder-1]
		stepResults[stepIndex] = &FunnelStepResult{
			StepOrder:      stepOrder,
			GoalName:       step.GoalName,
			Visitors:       visitors,
			DropOff:        0,
			ConversionRate: 0,
		}
		stepIndex++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate funnel results: %w", err)
	}

	// 计算 drop-off 和转化率
	for i := 1; i < len(stepResults); i++ {
		if stepResults[i] != nil && stepResults[i-1] != nil {
			stepResults[i].DropOff = int64(stepResults[i-1].Visitors) - int64(stepResults[i].Visitors)
		}
	}

	// 计算每步转化率（基于第一步）
	for i := 0; i < len(stepResults); i++ {
		if stepResults[i] != nil && totalVisitors > 0 {
			stepResults[i].ConversionRate = float64(stepResults[i].Visitors) / float64(totalVisitors) * 100
		}
	}

	// 计算整体转化率
	var conversionRate float64
	if totalVisitors > 0 && len(stepResults) > 0 && stepResults[len(stepResults)-1] != nil {
		conversionRate = float64(stepResults[len(stepResults)-1].Visitors) / float64(totalVisitors) * 100
	}

	return &FunnelAnalysisResult{
		TotalVisitors:  totalVisitors,
		Steps:          stepResults,
		ConversionRate: conversionRate,
	}, nil
}

// buildFunnelQuery 构建漏斗分析 SQL 查询。
// 策略：使用 CTE (Common Table Expression) 和窗口函数
// 对于 sequential 漏斗：每个步骤独立统计，允许其他活动在步骤之间发生
func (s *AnalysisService) buildFunnelQuery(req *AnalysisRequest) (string, []any) {
	args := []any{req.SiteID, req.StartTime, req.EndTime}

	// 构建每个步骤的 CTE
	withClauses := make([]string, len(req.Steps))
	stepNames := make([]string, len(req.Steps))

	for i, step := range req.Steps {
		stepName := fmt.Sprintf("step_%d", i+1)
		stepNames[i] = stepName

		// 根据 goal 类型构建不同的 WHERE 条件
		whereCondition := s.buildGoalCondition(step, i+2) // 参数从 3 开始

		withClauses[i] = fmt.Sprintf(`%s AS (
			SELECT user_id
			FROM zenstats_events_db.events
			WHERE site_id = toUInt64($1)
				AND timestamp >= $2
				AND timestamp <= $3
				AND %s
			GROUP BY user_id
		)`, stepName, whereCondition)

		args = append(args, step.GoalValue)
	}

	// 构建最终查询：计算每个步骤的用户数
	// 使用 INTERSECT 来确保用户按顺序完成所有前面的步骤
	selectParts := make([]string, len(req.Steps))
	for i := range req.Steps {
		stepName := stepNames[i]

		// 对于第 i 个步骤，需要与前面所有步骤取交集
		if i == 0 {
			selectParts[i] = fmt.Sprintf(`SELECT 1 AS step_order, count(*) AS visitors FROM %s`, stepName)
		} else {
			// 与前面所有步骤取交集
			intersectParts := make([]string, i+1)
			for j := 0; j <= i; j++ {
				intersectParts[j] = stepNames[j]
			}
			selectParts[i] = fmt.Sprintf(`SELECT %d AS step_order, count(*) AS visitors FROM %s`,
				i+1, joinWithIntersect(intersectParts))
		}
	}

	query := fmt.Sprintf(`WITH %s %s`,
		joinWithComma(withClauses),
		joinWithUnion(selectParts))

	return query, args
}

// buildGoalCondition 根据 goal 类型构建 WHERE 条件
func (s *AnalysisService) buildGoalCondition(step *FunnelStep, paramIndex int) string {
	switch step.GoalType {
	case "event":
		return fmt.Sprintf(`name = $%d`, paramIndex)
	case "page":
		return fmt.Sprintf(`pathname = $%d`, paramIndex)
	default:
		return fmt.Sprintf(`name = $%d`, paramIndex)
	}
}

// joinWithIntersect 使用 INTERSECT ALL 连接多个查询
func joinWithIntersect(parts []string) string {
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result = fmt.Sprintf(`%s INTERSECT ALL SELECT user_id FROM %s`, result, parts[i])
	}
	// 包装成子查询
	return fmt.Sprintf(`(SELECT user_id FROM %s)`, result)
}

// joinWithComma 用逗号连接字符串
func joinWithComma(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ", "
		}
		result += part
	}
	return result
}

// joinWithUnion 用 UNION ALL 连接查询
func joinWithUnion(parts []string) string {
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result = fmt.Sprintf(`%s UNION ALL %s`, result, parts[i])
	}
	return result
}
