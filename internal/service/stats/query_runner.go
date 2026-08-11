package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/zenstats/zenstats/internal/service/stats/sql"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

// malformedQueryCHCodes 是表示"查询构造本身非法"的 ClickHouse 错误码集合。
// 这类错误意味着请求的指标/维度组合不受支持（而非临时故障），应回 400 友好提示而非 500 泄露 SQL。
var malformedQueryCHCodes = map[int32]bool{
	10:  true, // NOT_FOUND_COLUMN_IN_BLOCK
	42:  true, // NUMBER_OF_ARGUMENTS_DOESNT_MATCH
	43:  true, // ILLEGAL_TYPE_OF_ARGUMENT
	44:  true, // ILLEGAL_COLUMN
	46:  true, // UNKNOWN_FUNCTION
	47:  true, // UNKNOWN_IDENTIFIER
	53:  true, // TYPE_MISMATCH
	60:  true, // UNKNOWN_TABLE
	184: true, // ILLEGAL_AGGREGATION
	215: true, // NOT_AN_AGGREGATE
	62:  true, // SYNTAX_ERROR
}

// translateQueryError 将 ClickHouse 的"非法查询"异常翻译为 400 校验错误（不支持的查询组合），
// 其余错误按查询执行失败返回，避免把原始 SQL 错误暴露给客户端。
func translateQueryError(err error) error {
	var chErr *clickhouse.Exception
	if errors.As(err, &chErr) && malformedQueryCHCodes[chErr.Code] {
		slog.Warn("unsupported query combination", "ch_code", chErr.Code, "ch_message", chErr.Message)
		return &ValidationError{
			StatusCode: 400,
			Message:    "the requested metric/dimension combination is not supported",
		}
	}
	return fmt.Errorf("query execution failed: %v", err)
}

// QueryRunner 执行统计查询并返回结果
type QueryRunner struct {
	conn driver.Conn
}

// NewQueryRunner 创建新的查询执行器
func NewQueryRunner() *QueryRunner {
	return &QueryRunner{conn: cl.GetConnection()}
}

// RunQuery 执行查询并返回结果
func (qr *QueryRunner) RunQuery(ctx context.Context, query *types.Query, site *types.Site) (*types.QueryResult, error) {
	// 创建查询构建器
	builder := sql.NewQueryBuilder()
	// 构建SQL
	sqlStr, args, err := builder.Build(query, site)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %v", err)
	}

	// 当需要分页时，先执行 COUNT 查询获取总行数
	var totalRows int
	if builder.NeedsTotalRows(query) {
		totalRows, err = qr.runCountQuery(ctx, sqlStr, args)
		if err != nil {
			slog.Warn("failed to count total rows", "error", err)
			totalRows = 0
		}
	}

	// 执行查询
	rows, err := qr.conn.Query(ctx, sqlStr, args...)
	slog.Debug("query sql", "sql", sqlStr, "args", args)

	if err != nil {
		return nil, translateQueryError(err)
	}
	defer rows.Close()
	// 处理结果
	result, err := qr.processResults(rows, query.Dimensions, builder)
	if err != nil {
		return nil, fmt.Errorf("failed to process results: %v", err)
	}

	// 设置分页元数据
	if builder.NeedsTotalRows(query) {
		result.TotalRows = totalRows
	}

	return result, nil
}

// runCountQuery 执行 COUNT 查询获取分页总行数
// 注意：必须移除原始 SQL 中的 LIMIT/OFFSET 子句，否则 COUNT 的只是截断后的行数。
func (qr *QueryRunner) runCountQuery(ctx context.Context, originalSQL string, args []any) (int, error) {
	// 移除 LIMIT / OFFSET 子句（大小写不敏感）
	cleanSQL := originalSQL
	// 查找最后一个 LIMIT 子句的位置
	limitIdx := strings.LastIndex(strings.ToUpper(cleanSQL), "LIMIT ")
	offsetIdx := strings.LastIndex(strings.ToUpper(cleanSQL), "OFFSET ")

	if limitIdx >= 0 {
		// 截掉 LIMIT 及其后面的所有内容（含 OFFSET，因为 OFFSET 总在 LIMIT 后）
		cleanSQL = cleanSQL[:limitIdx]
	} else if offsetIdx >= 0 {
		// 只有 OFFSET 没有 LIMIT（不常见，但 ClickHouse 允许）
		cleanSQL = cleanSQL[:offsetIdx]
	}

	countSQL := fmt.Sprintf("SELECT count(*) FROM (%s)", strings.TrimSpace(cleanSQL))

	var totalRows uint64
	err := qr.conn.QueryRow(ctx, countSQL, args...).Scan(&totalRows)
	if err != nil {
		return 0, err
	}
	return int(totalRows), nil
}

// processResults 处理查询结果
func (qr *QueryRunner) processResults(rows driver.Rows, dimensions []string, builder *sql.QueryBuilder) (*types.QueryResult, error) {
	// 获取列信息
	originalColumns := rows.Columns()
	newColumns := make([]string, len(originalColumns))
	copy(newColumns, originalColumns)

	// 如果只有一个维度，将对应列名改为"name"
	if len(dimensions) == 1 {
		targetDimension := dimensions[0]
		alias := builder.DimensionAlias(targetDimension)
		for i, col := range originalColumns {
			if col == alias {
				newColumns[i] = "name"
				break
			}
		}
	}

	// 准备结果容器
	var results []map[string]any

	// 列类型在所有行中不变：提前取一次并预计算每列的 kind，避免在行循环内
	// 重复调用 rows.ColumnTypes() 并对 DatabaseTypeName() 做两遍字符串 switch。
	columnTypes := rows.ColumnTypes()
	kinds := make([]colKind, len(columnTypes))
	for i, ct := range columnTypes {
		kinds[i] = classifyColumnKind(ct.DatabaseTypeName())
	}

	// 遍历行
	for rows.Next() {
		// 创建行数据容器
		row := make(map[string]any)

		// 为每一列创建适当类型的指针（按预计算 kind）
		values := make([]any, len(originalColumns))
		pointers := make([]any, len(originalColumns))
		for i := range kinds {
			switch kinds[i] {
			case kindUint8:
				var val uint8
				values[i] = &val
				pointers[i] = &val
			case kindUint16:
				var val uint16
				values[i] = &val
				pointers[i] = &val
			case kindUint32:
				var val uint32
				values[i] = &val
				pointers[i] = &val
			case kindUint64:
				var val uint64
				values[i] = &val
				pointers[i] = &val
			case kindInt32:
				var val int32
				values[i] = &val
				pointers[i] = &val
			case kindInt64:
				var val int64
				values[i] = &val
				pointers[i] = &val
			case kindFloat64:
				var val float64
				values[i] = &val
				pointers[i] = &val
			case kindString:
				var val string
				values[i] = &val
				pointers[i] = &val
			default:
				var val any
				values[i] = &val
				pointers[i] = &val
			}
		}

		// 扫描行数据
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}

		// 处理每一列数据
		for i, col := range newColumns {
			var val any
			switch kinds[i] {
			case kindUint8:
				val = *values[i].(*uint8)
			case kindUint16:
				val = *values[i].(*uint16)
			case kindUint32:
				val = *values[i].(*uint32)
			case kindUint64:
				val = *values[i].(*uint64)
			case kindInt32:
				val = *values[i].(*int32)
			case kindInt64:
				val = *values[i].(*int64)
			case kindFloat64:
				val = *values[i].(*float64)
			case kindString:
				val = *values[i].(*string)
			default:
				val = *values[i].(*any)
			}
			row[col] = convertValue(val)
		}

		results = append(results, row)
	}

	// 检查是否有错误
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &types.QueryResult{
		Columns: newColumns,
		Data:    results,
	}, nil
}

// colKind 是列的 Go 目标类型分类，供 processResults 预计算以避免逐行字符串判定。
type colKind uint8

const (
	kindAny colKind = iota
	kindUint8
	kindUint16
	kindUint32
	kindUint64
	kindInt32
	kindInt64
	kindFloat64
	kindString
)

// classifyColumnKind 将 ClickHouse DatabaseTypeName() 映射为 colKind，
// 逻辑与原逐行 switch 完全一致（先精确匹配，再按子串回退）。
func classifyColumnKind(typeName string) colKind {
	switch typeName {
	case "UInt8":
		return kindUint8
	case "UInt16":
		return kindUint16
	case "UInt32":
		return kindUint32
	case "UInt64":
		return kindUint64
	case "Int32":
		return kindInt32
	case "Int64":
		return kindInt64
	case "Float64", "Decimal":
		return kindFloat64
	case "String", "LowCardinality(String)":
		return kindString
	}
	switch {
	case strings.Contains(typeName, "Int64"):
		return kindInt64
	case strings.Contains(typeName, "Int32") || strings.Contains(typeName, "Int16") || strings.Contains(typeName, "Int8"):
		return kindInt32
	case strings.Contains(typeName, "UInt64"):
		return kindUint64
	case strings.Contains(typeName, "UInt32") || strings.Contains(typeName, "UInt16") || strings.Contains(typeName, "UInt8"):
		return kindUint32
	case strings.Contains(typeName, "Float") || strings.Contains(typeName, "Decimal"):
		return kindFloat64
	case strings.Contains(typeName, "String") || strings.Contains(typeName, "FixedString") || strings.Contains(typeName, "Enum"):
		return kindString
	default:
		return kindAny
	}
}

// convertValue 转换ClickHouse特定类型为通用类型
func convertValue(val any) any {
	// 处理不同类型的值
	switch v := val.(type) {
	case []byte:
		// 尝试解析JSON
		var jsonVal any
		if err := json.Unmarshal(v, &jsonVal); err == nil {
			return jsonVal
		}
		return string(v)
	case float64:
		// 兜底：NaN/±Inf 无法被 JSON 序列化，会导致响应体为空。归零保证结果始终可序列化。
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0.0
		}
		return v
	case float32:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0.0
		}
		return v
	default:
		return val
	}
}
