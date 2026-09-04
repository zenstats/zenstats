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

// NewAnalysisService 创建漏斗分析服务instance.
func NewAnalysisService() *AnalysisService {
	return &AnalysisService{
		conn: cl.GetConnection(),
	}
}

// FunnelStepResult 漏斗步骤分析结果（线性视图，相对第一步）。
type FunnelStepResult struct {
	StepOrder      int     `json:"step_order"`
	GoalName       string  `json:"goal_name"`
	Visitors       uint64  `json:"visitors"`
	DropOff        int64   `json:"drop_off"`
	ConversionRate float64 `json:"conversion_rate"`
}

// FunnelNodeResult 树形节点结果：按"从根到该节点的完整路径、时间严格递增"统计，转化率相对父节点。
type FunnelNodeResult struct {
	StepID         int64   `json:"step_id"`
	ParentStepID   *int64  `json:"parent_step_id,omitempty"`
	StepOrder      int     `json:"step_order"`
	GoalName       string  `json:"goal_name"`
	Visitors       uint64  `json:"visitors"`
	DropOff        int64   `json:"drop_off"`        // 父节点访客 - 本节点访客
	ConversionRate float64 `json:"conversion_rate"` // 相对父节点；根为 100
	IsRoot         bool    `json:"is_root"`
}

// FunnelAnalysisResult 漏斗分析结果。
type FunnelAnalysisResult struct {
	TotalVisitors  uint64              `json:"total_visitors"`
	Steps          []*FunnelStepResult `json:"steps"`
	ConversionRate float64             `json:"conversion_rate"`
	// Nodes 为树形视图数据：每个节点带 step_id/parent_step_id，前端据此重建多层树。
	Nodes []*FunnelNodeResult `json:"nodes"`
}

// FunnelStep 漏斗步骤定义。
type FunnelStep struct {
	StepID       int64
	ParentStepID *int64 // 父步骤 ID；根为 nil。用于树形路径分析。
	GoalID       int64
	GoalType     string // "event" or "page"
	GoalValue    string // event name or page path
	GoalName     string
	StepOrder    int
	CustomProps  map[string]string // 目标自定义属性过滤条件
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

	// 处理结果 — 按 stepOrder 索引，确保顺序不受 SQL 返回行序影响。
	// 每行即一个节点按其根路径统计的访客数。
	for rows.Next() {
		var visitors uint64
		var stepOrder uint8

		if err := rows.Scan(&stepOrder, &visitors); err != nil {
			return nil, fmt.Errorf("failed to scan funnel result: %w", err)
		}
		if int(stepOrder) < 1 || int(stepOrder) > len(req.Steps) {
			continue
		}

		if stepOrder == 1 {
			totalVisitors = visitors
		}

		step := req.Steps[stepOrder-1]
		stepResults[stepOrder-1] = &FunnelStepResult{
			StepOrder:      int(stepOrder),
			GoalName:       step.GoalName,
			Visitors:       visitors,
			DropOff:        0,
			ConversionRate: 0,
		}
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

	// 组装树形节点：每个节点的转化率/流失相对其父节点（根相对自身为 100%）。
	parent := parentIndices(req.Steps)
	nodes := make([]*FunnelNodeResult, 0, len(req.Steps))
	for i, step := range req.Steps {
		var visitors uint64
		if stepResults[i] != nil {
			visitors = stepResults[i].Visitors
		}
		parentVisitors := totalVisitors
		isRoot := parent[i] == -1
		if !isRoot && stepResults[parent[i]] != nil {
			parentVisitors = stepResults[parent[i]].Visitors
		}
		var cr float64
		if isRoot {
			cr = 100
		} else if parentVisitors > 0 {
			cr = float64(visitors) / float64(parentVisitors) * 100
		}
		var dropOff int64
		if !isRoot {
			dropOff = int64(parentVisitors) - int64(visitors)
		}
		nodes = append(nodes, &FunnelNodeResult{
			StepID:         step.StepID,
			ParentStepID:   step.ParentStepID,
			StepOrder:      step.StepOrder,
			GoalName:       step.GoalName,
			Visitors:       visitors,
			DropOff:        dropOff,
			ConversionRate: cr,
			IsRoot:         isRoot,
		})
	}

	return &FunnelAnalysisResult{
		TotalVisitors:  totalVisitors,
		Steps:          stepResults,
		ConversionRate: conversionRate,
		Nodes:          nodes,
	}, nil
}

// buildFunnelQuery 构建漏斗分析 SQL 查询。
// 策略：单次扫描事件表，使用 minIf 按步骤条件计算每个用户各步骤的最早触发时间，
// 再通过 WHERE 比较时间戳确保严格时序。避免多表 JOIN 的列引用问题，性能更优。
func (s *AnalysisService) buildFunnelQuery(req *AnalysisRequest) (string, []any) {
	// 参数顺序必须匹配最终 SQL 中 ? 的出现顺序：
	// minIf 中的 ? (SELECT 子句，最先) → site_id/timestamp 的 ? (WHERE 子句) → WHERE OR 中的 ?
	// 因此先把各 step 的 goal value 放入 args，再放 site_id / timestamps
	var args []any
	stepConds := make([]string, len(req.Steps))
	orConds := make([]string, len(req.Steps))
	var goalArgs []any
	for i, step := range req.Steps {
		cond := s.buildGoalCondition(step)
		stepConds[i] = cond
		orConds[i] = cond
		val := step.GoalValue
		if hasWildcard(val) {
			val = wildcardToClickHouseRegex(val)
		}
		goalArgs = append(goalArgs, val)
	}
	// minIf 中的 ? (每个 step 一次) + site_id/timestamps (3个) + WHERE OR 中的 ? (每个 step 一次)
	args = append(args, goalArgs...)                            // minIf 部分
	args = append(args, req.SiteID, req.StartTime, req.EndTime) // WHERE 条件
	args = append(args, goalArgs...)                            // WHERE OR 部分

	// CTE: 单表扫描，每个用户一行，含各步骤的最早时间
	minIfParts := make([]string, len(req.Steps))
	for i := range req.Steps {
		minIfParts[i] = fmt.Sprintf(`minIf(timestamp, %s) AS ts_%d`, stepConds[i], i+1)
	}

	// 每个节点按"从根到该节点的完整路径、时间严格递增"统计。
	// 线性漏斗（父=上一节点）时路径即顺序链，等价于原严格时序漏斗；
	// 树形漏斗时每个分支按各自路径统计，天然支持任意分叉与多层。
	parent := parentIndices(req.Steps)
	selectParts := make([]string, len(req.Steps))
	for i := range req.Steps {
		conds := pathConditions(parent, i)
		selectParts[i] = fmt.Sprintf(
			`SELECT %d AS step_order, count(*) AS visitors FROM funnel_data WHERE %s`,
			i+1, strings.Join(conds, " AND "),
		)
	}

	query := fmt.Sprintf(
		`WITH funnel_data AS (
			SELECT user_id, %s
			FROM zenstats_events_db.events
			WHERE site_id = toUInt64(?)
				AND timestamp >= ?
				AND timestamp <= ?
				AND (%s)
			GROUP BY user_id
		) %s`,
		strings.Join(minIfParts, ", "),
		strings.Join(orConds, " OR "),
		strings.Join(selectParts, " UNION ALL "),
	)

	return query, args
}

// parentIndices 把每个步骤的 ParentStepID 映射为其在数组中的下标；根返回 -1。
// 找不到父（数据异常）也按根处理，避免悬挂引用。
func parentIndices(steps []*FunnelStep) []int {
	idOf := make(map[int64]int, len(steps))
	for i, s := range steps {
		idOf[s.StepID] = i
	}
	parent := make([]int, len(steps))
	for i, s := range steps {
		parent[i] = -1
		if s.ParentStepID != nil {
			if p, ok := idOf[*s.ParentStepID]; ok && p != i {
				parent[i] = p
			}
		}
	}
	return parent
}

// pathConditions 返回节点 i 从根到自身路径的 WHERE 条件：
// 路径上每个节点的 ts 非空，且沿路径时间严格递增（子 > 父）。
// 用 visited 保护，即使父引用异常也不会死循环。
func pathConditions(parent []int, i int) []string {
	var path []int
	visited := make(map[int]bool)
	for j := i; j != -1 && !visited[j]; j = parent[j] {
		visited[j] = true
		path = append([]int{j}, path...) // 前插，最终 path[0] 为根
	}
	if len(path) == 0 {
		return []string{"1 = 1"}
	}
	conds := []string{fmt.Sprintf(`ts_%d IS NOT NULL`, path[0]+1)}
	for k := 1; k < len(path); k++ {
		conds = append(conds,
			fmt.Sprintf(`ts_%d IS NOT NULL`, path[k]+1),
			fmt.Sprintf(`ts_%d > ts_%d`, path[k]+1, path[k-1]+1),
		)
	}
	return conds
}

// buildGoalCondition 根据 goal 类型构建 WHERE 条件。
//
// 页面类型目标：仅匹配 pageview 事件在指定路径上的发生。
// 事件类型目标：按事件名称匹配。
//
// 支持两种匹配模式：
//   - 精确匹配：goal 值不含 * 或 ? 时，使用 = 比较
//   - 通配符匹配：goal 值含 * 或 ? 时，使用 ClickHouse match() 正则匹配
//     例如 /soft/*.html 会匹配 /soft/1111.html、/soft/abc.html 等
func (s *AnalysisService) buildGoalCondition(step *FunnelStep) string {
	var baseCond string
	if step.GoalType == "page" {
		// 页面类型目标：过滤到 pageview 事件 + 匹配 pathname
		if hasWildcard(step.GoalValue) {
			baseCond = `(name = 'pageview' AND match(pathname, ?))`
		} else {
			baseCond = `(name = 'pageview' AND pathname = ?)`
		}
	} else {
		// 事件类型目标：按事件名称匹配
		if hasWildcard(step.GoalValue) {
			baseCond = `match(name, ?)`
		} else {
			baseCond = `name = ?`
		}
	}

	// 追加自定义属性过滤条件
	// key/val 来自用户配置的目标自定义属性，必须转义反斜杠与单引号，
	// 防止值中的引号闭合 SQL 字符串导致 ClickHouse 注入（可跨租户读取）。
	if len(step.CustomProps) > 0 {
		for key, val := range step.CustomProps {
			baseCond += fmt.Sprintf(
				` AND "meta.value"[indexOf("meta.key", '%s')] = '%s'`,
				escapeCHString(key), escapeCHString(val),
			)
		}
	}

	return baseCond
}

// escapeCHString 转义反斜杠与单引号，用于安全嵌入 ClickHouse 字符串字面量。
func escapeCHString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

// hasWildcard 检查字符串是否包含通配符 * 或 ?
func hasWildcard(s string) bool {
	return strings.Contains(s, "*") || strings.Contains(s, "?")
}

// wildcardToClickHouseRegex 将通配符模式转换为 ClickHouse match() 可用的正则表达式。
//
//   - → .*  (匹配任意字符任意次)
//     ? → .   (匹配单个字符)
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

	// 添加锚点确保全串匹配，与 ClickHouse match() FullMatch 语义一致
	return "^" + escaped + "$"
}
