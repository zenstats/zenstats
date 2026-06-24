package types

import (
	"encoding/json"
	"errors"
	"strings"
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
		subArr, err := normalizeSubFilters(raw[1])
		if err != nil {
			return nil, err
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

// ParseRawFiltersJSON 从 GET 查询参数中的 JSON 字符串解析过滤器。
// 两种常用格式：
//   - [["is", "visit:browser", ["Chrome"]]]
//   - ["and", [["is", "visit:browser", ["Chrome"]], ["is", "visit:country", ["CN"]]]]
func ParseRawFiltersJSON(raw string) ([]*Filter, error) {
	if raw == "" {
		return nil, nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}

	arr, ok := decoded.([]any)
	if !ok || len(arr) == 0 {
		return nil, errors.New("filters must be a non-empty array")
	}

	if op, ok := arr[0].(string); ok && (op == "and" || op == "or") {
		filter, err := ParseRawFilter(arr)
		if err != nil {
			return nil, err
		}
		return []*Filter{filter}, nil
	}

	filters := make([]*Filter, 0, len(arr))
	for _, item := range arr {
		filterArr, ok := item.([]any)
		if !ok {
			return nil, errors.New("filter item must be array")
		}
		filter, err := ParseRawFilter(filterArr)
		if err != nil {
			return nil, err
		}
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	return filters, nil
}

func normalizeSubFilters(value any) ([][]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("subfilters must be array")
	}

	result := make([][]any, 0, len(items))
	for _, item := range items {
		arr, ok := item.([]any)
		if !ok {
			return nil, errors.New("subfilter must be array")
		}
		result = append(result, arr)
	}
	return result, nil
}

// Clone returns a deep-enough copy of the filter tree for query pipeline rewrites.
func (f *Filter) Clone() *Filter {
	if f == nil {
		return nil
	}
	clone := *f
	if f.Modifiers != nil {
		clone.Modifiers = make(map[string]string, len(f.Modifiers))
		for k, v := range f.Modifiers {
			clone.Modifiers[k] = v
		}
	}
	if f.Values != nil {
		clone.Values = append([]any(nil), f.Values...)
	}
	if f.SubFilters != nil {
		clone.SubFilters = make([]*Filter, 0, len(f.SubFilters))
		for _, sub := range f.SubFilters {
			clone.SubFilters = append(clone.SubFilters, sub.Clone())
		}
	}
	return &clone
}

// AnyDimension reports whether any leaf filter dimension matches predicate.
func (f *Filter) AnyDimension(predicate func(string) bool) bool {
	if f == nil {
		return false
	}
	if f.Dimension != "" && predicate(f.Dimension) {
		return true
	}
	for _, sub := range f.SubFilters {
		if sub.AnyDimension(predicate) {
			return true
		}
	}
	return false
}

// RewriteDimensions returns a cloned filter tree with dimensions rewritten by mapper.
func (f *Filter) RewriteDimensions(mapper func(string) string) *Filter {
	clone := f.Clone()
	var rewrite func(*Filter)
	rewrite = func(node *Filter) {
		if node == nil {
			return
		}
		if node.Dimension != "" {
			node.Dimension = mapper(node.Dimension)
		}
		for _, sub := range node.SubFilters {
			rewrite(sub)
		}
	}
	rewrite(clone)
	return clone
}

// HasEventGoalFilter detects goal filters recursively.
func HasEventGoalFilter(filters []*Filter) bool {
	for _, f := range filters {
		if f.AnyDimension(func(dim string) bool { return dim == "event:goal" }) {
			return true
		}
	}
	return false
}

// HasDimensionPrefix detects dimensions recursively by prefix.
func HasDimensionPrefix(filters []*Filter, prefix string) bool {
	for _, f := range filters {
		if f.AnyDimension(func(dim string) bool { return strings.HasPrefix(dim, prefix) }) {
			return true
		}
	}
	return false
}
