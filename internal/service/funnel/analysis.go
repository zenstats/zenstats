package funnel

import (
	"context"
	"fmt"
	"strings"
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
		var stepOrder uint8

		if err := rows.Scan(&stepOrder, &visitors); err != nil {
			return nil, fmt.Errorf("failed to scan funnel result: %w", err)
		}

		if stepIndex == 0 {
			totalVisitors = visitors
		}

		step := req.Steps[stepOrder-1]
		stepResults[stepIndex] = &FunnelStepResult{
			StepOrder:      int(stepOrder),
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
// 策略：单次扫描事件表，使用 minIf 按步骤条件计算每个用户各步骤的最早触发时间，
// 再通过 WHERE 比较时间戳确保严格时序。避免多表 JOIN 的列引用问题，性能更优。
func (s *AnalysisService) buildFunnelQuery(req *AnalysisRequest) (string, []any) {
	args := []any{req.SiteID, req.StartTime, req.EndTime}

	// 构建 per-step 条件表达式（用于 minIf 和 WHERE OR）
	stepConds := make([]string, len(req.Steps))
	orConds := make([]string, len(req.Steps))
	for i, step := range req.Steps {
		cond := s.buildGoalCondition(step, i+4)
		stepConds[i] = cond
		orConds[i] = cond
		// 通配符模式下将 * 转换为 ClickHouse 正则 .*
		val := step.GoalValue
		if hasWildcard(val) {
			val = wildcardToClickHouseRegex(val)
		}
		args = append(args, val)
	}

	// CTE: 单表扫描，每个用户一行，含各步骤的最早时间
	minIfParts := make([]string, len(req.Steps))
	for i := range req.Steps {
		minIfParts[i] = fmt.Sprintf(`minIf(timestamp, %s) AS ts_%d`, stepConds[i], i+1)
	}

	// 构建每个步骤的统计 SELECT
	selectParts := make([]string, len(req.Steps))
	for i := range req.Steps {
		// 步骤 N 的 WHERE：前 N 步时间非空且严格递增
		conds := make([]string, 0, i+1)
		for j := 0; j <= i; j++ {
			conds = append(conds, fmt.Sprintf(`ts_%d IS NOT NULL`, j+1))
			if j > 0 {
				conds = append(conds, fmt.Sprintf(`ts_%d > ts_%d`, j+1, j))
			}
		}
		selectParts[i] = fmt.Sprintf(
			`SELECT %d AS step_order, count(*) AS visitors FROM funnel_data WHERE %s`,
			i+1, strings.Join(conds, " AND "),
		)
	}

	query := fmt.Sprintf(
		`WITH funnel_data AS (
			SELECT user_id, %s
			FROM zenstats_events_db.events
			WHERE site_id = toUInt64($1)
				AND timestamp >= $2
				AND timestamp <= $3
				AND (%s)
			GROUP BY user_id
		) %s`,
		strings.Join(minIfParts, ", "),
		strings.Join(orConds, " OR "),
		strings.Join(selectParts, " UNION ALL "),
	)

	return query, args
}

// buildGoalCondition 根据 goal 类型构建 WHERE 条件。
// 支持两种匹配模式：
//   - 精确匹配：goal 值不含 * 或 ? 时，使用 = 比较
//   - 通配符匹配：goal 值含 * 或 ? 时，使用 ClickHouse match() 正则匹配
//     例如 /soft/*.html 会匹配 /soft/1111.html、/soft/abc.html 等
func (s *AnalysisService) buildGoalCondition(step *FunnelStep, paramIndex int) string {
	col := "name"
	if step.GoalType == "page" {
		col = "pathname"
	}
	if hasWildcard(step.GoalValue) {
		// 通配符 → 正则匹配
		return fmt.Sprintf(`match(%s, $%d)`, col, paramIndex)
	}
	return fmt.Sprintf(`%s = $%d`, col, paramIndex)
}

// hasWildcard 检查字符串是否包含通配符 * 或 ?
func hasWildcard(s string) bool {
	return strings.Contains(s, "*") || strings.Contains(s, "?")
}

// wildcardToClickHouseRegex 将通配符模式转换为 ClickHouse match() 可用的正则表达式。
//
//	* → .*  (匹配任意字符任意次)
//	? → .   (匹配单个字符)
//
// 其他正则特殊字符会被转义。
// 例如 /soft/*.html → /soft/.*\.html
func wildcardToClickHouseRegex(pattern string) string {
	// 先转义正则特殊字符（除了 * 和 ?）
	re := strings.NewReplacer(
		`.`, `\.`,
		`+`, `\+`,
		`(`, `\(`,
		`)`, `\)`,
		`[`, `\[`,
		`]`, `\]`,
		`{`, `\{`,
		`}`, `\}`,
		`^`, `\^`,
		`$`, `\$`,
		`|`, `\|`,
	)
	escaped := re.Replace(pattern)

	// * → .*, ? → .
	escaped = strings.ReplaceAll(escaped, "*", ".*")
	escaped = strings.ReplaceAll(escaped, "?", ".")

	return escaped
}
