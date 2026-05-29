package types

import (
	"errors"
)

// Filter 表示查询中的一个过滤条件，支持基础比较和复合逻辑运算。
type Filter struct {
	Operator   string            `json:"operator"`  // 基础比较: is, is_not 模式匹配: matches, matches_wildcard 包含关系: contains, contains_not
	Dimension  string            `json:"dimension"` //
	Values     []any             `json:"values"`
	Modifiers  map[string]string `json:"modifiers"`
	SubFilters []*Filter         `json:"sub_filters"`
}

// [操作符, 维度, 值列表, 修饰符]
// # 基础过滤器
// [:is, "visit:browser", ["Chrome"]]
// [:is_not, "event:page", ["/admin"]]

// # 复合过滤器
// [:and, [
//   [:is, "visit:country", ["US"]],
//   [:or, [
//     [:is, "visit:browser", ["Chrome"]],
//     [:is, "visit:browser", ["Firefox"]]
//   ]]
// ]]

// ParseRawFilter 从原始 JSON 数组解析过滤条件，支持 and/or 复合条件嵌套。
func ParseRawFilter(raw []any) (*Filter, error) {

	if len(raw) == 0 {
		return nil, nil
	}

	op, ok := raw[0].(string)
	if !ok {
		return nil, errors.New("operator must be string")
	}

	switch op {
	case "and", "or":
		// 复合过滤器，第二项是子过滤器数组
		if len(raw) < 2 {
			return nil, errors.New("compound filter missing subfilters")
		}
		subArr, ok := raw[1].([][]any)
		if !ok {
			return nil, errors.New("subfilters must be array")
		}
		var subFilters []*Filter
		for _, sub := range subArr {
			f, err := ParseRawFilter(sub)
			if err != nil {
				return nil, err
			}
			subFilters = append(subFilters, f)
		}
		return &Filter{
			Operator:   op,
			SubFilters: subFilters,
		}, nil
	default:
		// 基础过滤器: [操作符, 维度, 值列表, (可选)修饰符]
		if len(raw) < 3 {
			return nil, errors.New("basic filter missing fields")
		}
		dim, _ := raw[1].(string)
		vals, _ := raw[2].([]any)
		var mods map[string]string
		if len(raw) > 3 {
			mods, _ = raw[3].(map[string]string)
		}
		return &Filter{
			Operator:  op,
			Dimension: dim,
			Values:    vals,
			Modifiers: mods,
		}, nil
	}
}
