package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/zenstats/zenstats/internal/service/stats/sql"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
)

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
	// 执行查询
	rows, err := qr.conn.Query(ctx, sqlStr, args...)
	slog.Debug("query sql", sqlStr, args)

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %v", err)
	}
	defer rows.Close()
	// 处理结果
	result, err := qr.processResults(rows, query.Dimensions, builder)
	if err != nil {
		return nil, fmt.Errorf("failed to process results: %v", err)
	}

	return result, nil
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
		convertedColumn := builder.DimensionToColumn(targetDimension, "events", "select")
		for i, col := range originalColumns {
			if col == convertedColumn {
				newColumns[i] = "name"
				break
			}
		}
	}

	// 准备结果容器
	var results []map[string]any

	// 遍历行
	for rows.Next() {
		// 创建行数据容器
		row := make(map[string]any)
		// 获取列类型信息
		columnTypes := rows.ColumnTypes()

		// 为每一列创建适当类型的指针
		values := make([]any, len(originalColumns))
		pointers := make([]any, len(originalColumns))
		for i, colType := range columnTypes {
			switch colType.DatabaseTypeName() {
			case "UInt8":
				var val uint8
				values[i] = &val
				pointers[i] = &val
			case "UInt16":
				var val uint16
				values[i] = &val
				pointers[i] = &val
			case "UInt32":
				var val uint32
				values[i] = &val
				pointers[i] = &val
			case "UInt64":
				var val uint64
				values[i] = &val
				pointers[i] = &val
			case "Int32":
				var val int32
				values[i] = &val
				pointers[i] = &val
			case "Int64":
				var val int64
				values[i] = &val
				pointers[i] = &val
			case "Float64", "Decimal":
				var val float64
				values[i] = &val
				pointers[i] = &val
			case "String", "LowCardinality(String)":
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
			switch columnTypes[i].DatabaseTypeName() {
			case "UInt8":
				val = *values[i].(*uint8)
			case "UInt16":
				val = *values[i].(*uint16)
			case "UInt32":
				val = *values[i].(*uint32)
			case "UInt64":
				val = *values[i].(*uint64)
			case "Int32":
				val = *values[i].(*int32)
			case "Int64":
				val = *values[i].(*int64)
			case "Float64", "Decimal":
				val = *values[i].(*float64)
			case "String", "LowCardinality(String)":
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
	default:
		return val
	}
}
