package sql

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/goal"
	"github.com/zenstats/zenstats/pkg/globals"
)

// WhereBuilder 构建查询的WHERE子句
type WhereBuilder struct {
	conditions []string
	params     []any
	siteID     string
	userID     int64
	timeRange  *types.TimeRange
}

// NewWhereBuilder 创建WhereBuilder实例
func NewWhereBuilder(siteID string) *WhereBuilder {
	return &WhereBuilder{siteID: siteID}
}

// Build 构建完整的WHERE子句
func (wb *WhereBuilder) Build() (string, []any) {
	if len(wb.conditions) == 0 {
		return "", nil
	}
	return strings.Join(wb.conditions, " AND "), wb.params
}

// AddCondition 添加条件到WHERE子句
func (wb *WhereBuilder) AddCondition(condition string, params ...any) {
	wb.conditions = append(wb.conditions, condition)
	wb.params = append(wb.params, params...)
}

// FilterSiteTimeRange 添加站点ID和时间范围过滤
func (wb *WhereBuilder) FilterSiteTimeRange(table string, firstDatetime, lastDatetime time.Time) {
	wb.timeRange = &types.TimeRange{Start: firstDatetime, End: lastDatetime}
	if table == "events" {
		wb.AddCondition(
			"site_id = ? AND timestamp >= ? AND timestamp <= ?",
			wb.siteID,
			firstDatetime,
			lastDatetime,
		)
	} else {
		// 会话表时间范围过滤，添加-7天偏移（模仿Elixir实现）
		sevenDaysBefore := firstDatetime.AddDate(0, 0, -7)
		condition := "site_id = ? AND start >= ? AND start <= ? AND timestamp >= ?"
		params := []any{wb.siteID, sevenDaysBefore, lastDatetime, firstDatetime}

		// 会话特有字段过滤
		if len(wb.conditions) > 0 {
			lastCondition := wb.conditions[len(wb.conditions)-1]
			if strings.Contains(lastCondition, "entry_page") || strings.Contains(lastCondition, "exit_page") {
				condition += " AND entry_page IS NOT NULL AND exit_page IS NOT NULL"
			}
		}

		wb.AddCondition(condition, params...)
	}
}

// AddFilter 根据过滤条件类型添加相应的WHERE子句
func (wb *WhereBuilder) AddFilter(table string, filter *types.Filter) error {
	if filter == nil {
		return nil
	}

	// Route a complete compatible subtree, rather than discarding foreign leaves.
	// Keeping the subtree intact preserves OR/NOT and same-event conjunctions.
	if target := foreignFilterTable(table, filter); target != "" {
		return wb.addSessionMembership(target, filter, false)
	}

	// 特殊维度处理
	switch filter.Dimension {
	case "event:name":
		return wb.addFilterField("name", filter)
	case "event:page":
		return wb.addFilterField("pathname", filter)
	case "event:hostname":
		return wb.addFilterField("hostname", filter)
	case "event:goal":
		return wb.addGoalFilter(filter)
	case "visit:source":
		clause, err := NewQueryBuilder().getSourceClause(wb.userID)
		if err != nil {
			return err
		}
		return wb.addFilterCondition("("+clause+")", filter)
	}

	// 自定义属性维度处理
	if strings.HasPrefix(filter.Dimension, "event:props:") || strings.HasPrefix(filter.Dimension, "visit:entry_props:") {
		return wb.addCustomPropFilter(filter)
	}

	switch filter.Operator {
	case "not":
		return wb.addNotFilter(table, filter)
	case "and":
		return wb.addAndFilter(table, filter)
	case "or":
		return wb.addOrFilter(table, filter)
	case "has_done":
		return wb.addHasDoneFilter(table, filter)
	case "has_not_done":
		return wb.addHasNotDoneFilter(table, filter)
	default:
		return wb.addSimpleFilter(table, filter)
	}
}

// newSubBuilder preserves tenant, user-specific source mappings and query bounds.
func (wb *WhereBuilder) newSubBuilder() *WhereBuilder {
	return &WhereBuilder{siteID: wb.siteID, userID: wb.userID, timeRange: wb.timeRange}
}

func pureFilterDimensions(filter *types.Filter, predicate func(string) bool) bool {
	if filter == nil {
		return false
	}
	if filter.Dimension != "" {
		return predicate(filter.Dimension)
	}
	if filter.Operator != "and" && filter.Operator != "or" && filter.Operator != "not" {
		return false
	}
	if len(filter.SubFilters) == 0 {
		return false
	}
	for _, sub := range filter.SubFilters {
		if !pureFilterDimensions(sub, predicate) {
			return false
		}
	}
	return true
}

func foreignFilterTable(table string, filter *types.Filter) string {
	if table == "sessions" && pureFilterDimensions(filter, func(dim string) bool {
		return strings.HasPrefix(dim, "event:") || dim == "visit:page"
	}) {
		return "events"
	}
	if table == "events" && pureFilterDimensions(filter, isSessionOnlyDimension) {
		return "sessions"
	}
	return ""
}

func (wb *WhereBuilder) addSessionMembership(table string, filter *types.Filter, negate bool) error {
	sub := wb.newSubBuilder()
	if wb.timeRange != nil {
		sub.FilterSiteTimeRange(table, wb.timeRange.Start, wb.timeRange.End)
	} else {
		sub.AddCondition("site_id = ?", wb.siteID)
	}
	if err := sub.AddFilter(table, filter); err != nil {
		return err
	}
	condition, params := sub.Build()
	from := table
	if table == "sessions" {
		from += " FINAL"
		sub.AddCondition("sign = 1")
		condition, params = sub.Build()
	}
	op := "IN"
	if negate {
		op = "NOT IN"
	}
	wb.AddCondition(fmt.Sprintf("session_id %s (SELECT session_id FROM %s WHERE %s)", op, from, condition), params...)
	return nil
}

// 添加Custom property filter
func (wb *WhereBuilder) addCustomPropFilter(filter *types.Filter) error {
	var column string
	var propName string

	if strings.HasPrefix(filter.Dimension, "event:props:") {
		column = "meta"
		propName = strings.TrimPrefix(filter.Dimension, "event:props:")
	} else if strings.HasPrefix(filter.Dimension, "visit:entry_props:") {
		column = "entry_meta"
		propName = strings.TrimPrefix(filter.Dimension, "visit:entry_props:")
	} else {
		return fmt.Errorf("invalid custom property dimension: %s", filter.Dimension)
	}

	fieldExpr := fmt.Sprintf("%s.value[indexOf(%s.key, '%s')]", column, column, escapeCHString(propName))
	exists := fmt.Sprintf("has(%s.key, ?)", column)
	if filter.Operator != "is" && filter.Operator != "is_not" {
		wb.AddCondition(exists, propName)
		return wb.addFilterCondition(fieldExpr, filter)
	}

	var values []any
	missing := false
	for _, value := range filter.Values {
		if value == "(none)" {
			missing = true
		} else {
			values = append(values, value)
		}
	}
	if len(values) == 0 && !missing {
		return fmt.Errorf("property filter requires at least one value")
	}
	parts := []string{}
	params := []any{}
	if missing {
		parts = append(parts, "NOT "+exists)
		params = append(params, propName)
	}
	if len(values) > 0 {
		sub := wb.newSubBuilder()
		match := *filter
		match.Operator = "is"
		match.Values = values
		if err := sub.addFilterCondition(fieldExpr, &match); err != nil {
			return err
		}
		condition, args := sub.Build()
		parts = append(parts, "("+exists+" AND "+condition+")")
		params = append(params, propName)
		params = append(params, args...)
	}
	condition := "(" + strings.Join(parts, " OR ") + ")"
	if filter.Operator == "is_not" {
		condition = "NOT " + condition
	}
	wb.AddCondition(condition, params...)
	return nil
}

// 添加字段过滤条件
func (wb *WhereBuilder) addFilterField(fieldName string, filter *types.Filter) error {
	// Add condition based on operator
	return wb.addFilterCondition(fieldName, filter)
}

// 添加通用过滤条件
func (wb *WhereBuilder) addFilterCondition(fieldExpr string, filter *types.Filter) error {
	switch filter.Operator {
	case "is":
		return wb.addIsFilter(fieldExpr, filter.Values, filter.Modifiers)
	case "is_not":
		return wb.addIsNotFilter(fieldExpr, filter.Values, filter.Modifiers)
	case "contains":
		return wb.addContainsFilter(fieldExpr, filter.Values, filter.Modifiers)
	case "contains_not":
		return wb.addContainsNotFilter(fieldExpr, filter.Values, filter.Modifiers)
	case "matches":
		return wb.addMatchesFilter(fieldExpr, filter.Values, filter.Modifiers)
	case "matches_not":
		return wb.addMatchesNotFilter(fieldExpr, filter.Values, filter.Modifiers)
	case "matches_wildcard":
		return wb.addMatchesWildcardFilter(fieldExpr, filter.Values, filter.Modifiers)
	case "matches_wildcard_not":
		return wb.addMatchesWildcardNotFilter(fieldExpr, filter.Values, filter.Modifiers)
	default:
		return fmt.Errorf("unsupported filter operator: %s", filter.Operator)
	}
}

// 实现各种过滤条件的方法...
func (wb *WhereBuilder) addNotFilter(table string, filter *types.Filter) error {
	if len(filter.SubFilters) == 0 {
		return fmt.Errorf("'not' filter requires at least one sub-filter")
	}
	subBuilder := wb.newSubBuilder()
	if err := subBuilder.AddFilter(table, filter.SubFilters[0]); err != nil {
		return err
	}
	condition, params := subBuilder.Build()
	if condition == "" {
		return nil
	}
	wb.AddCondition(fmt.Sprintf("NOT (%s)", condition), params...)
	return nil
}

func (wb *WhereBuilder) addAndFilter(table string, filter *types.Filter) error {
	subBuilder := wb.newSubBuilder()
	foreign := map[string][]*types.Filter{}
	for _, subFilter := range filter.SubFilters {
		if target := foreignFilterTable(table, subFilter); target != "" {
			foreign[target] = append(foreign[target], subFilter)
			continue
		}
		if err := subBuilder.AddFilter(table, subFilter); err != nil {
			return err
		}
	}
	for _, target := range []string{"events", "sessions"} {
		if children := foreign[target]; len(children) > 0 {
			if err := subBuilder.addSessionMembership(target, &types.Filter{Operator: "and", SubFilters: children}, false); err != nil {
				return err
			}
		}
	}
	condition, params := subBuilder.Build()
	if condition != "" {
		wb.AddCondition("("+condition+")", params...)
	}
	return nil
}

func (wb *WhereBuilder) addOrFilter(table string, filter *types.Filter) error {
	conditions := []string{}
	allParams := []any{}

	for _, subFilter := range filter.SubFilters {
		subBuilder := wb.newSubBuilder()
		if err := subBuilder.AddFilter(table, subFilter); err != nil {
			return err
		}
		condition, params := subBuilder.Build()
		if condition != "" {
			conditions = append(conditions, "("+condition+")")
			allParams = append(allParams, params...)
		}
	}

	if len(conditions) == 0 {
		return nil
	}

	wb.AddCondition(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), allParams...)
	return nil
}

func (wb *WhereBuilder) addHasDoneFilter(table string, filter *types.Filter) error {
	if len(filter.SubFilters) != 1 || filter.SubFilters[0] == nil {
		return fmt.Errorf("'has_done' filter requires one sub-filter")
	}
	return wb.addSessionMembership("events", filter.SubFilters[0], false)
}

func (wb *WhereBuilder) addHasNotDoneFilter(table string, filter *types.Filter) error {
	if len(filter.SubFilters) != 1 || filter.SubFilters[0] == nil {
		return fmt.Errorf("'has_not_done' filter requires one sub-filter")
	}
	return wb.addSessionMembership("events", filter.SubFilters[0], true)
}

func (wb *WhereBuilder) addSimpleFilter(table string, filter *types.Filter) error {
	// 根据字段类型和过滤操作添加条件
	fieldName, err := wb.dbFieldName(filter.Dimension)
	if err != nil {
		return err
	}

	return wb.addFilterCondition(fieldName, filter)
}

// 字段名映射和值转换
func (wb *WhereBuilder) dbFieldName(name string) (string, error) {
	// Event aliases share physical columns with visit dimensions.
	if strings.HasPrefix(name, "event:") {
		switch name {
		case "event:referrerdomain":
			return "referrer_source", nil
		case "event:referrer":
			return "referrer", nil
		case "event:country", "event:browser", "event:browser_version", "event:os", "event:os_version", "event:device", "event:screen_size":
			name = "visit:" + strings.TrimPrefix(name, "event:")
		}
	}
	switch name {
	case "channel", "visit:channel":
		return "channel", nil
	case "event:name":
		return "name", nil
	case "event:goal":
		return "goal", nil
	case "event:page", "visit:page":
		return "pathname", nil
	case "event:hostname":
		return "hostname", nil
	case "visit:source":
		return "referrer_source", nil
	case "visit:medium":
		return "referrer_medium", nil
	case "visit:referrer":
		return "referrer", nil
	case "visit:entry_page":
		return "entry_page", nil
	case "visit:exit_page":
		return "exit_page", nil
	case "visit:entry_page_hostname", "visit:exit_page_hostname":
		return "hostname", nil
	case "visit:country":
		return "country_code", nil
	case "event:region", "visit:region":
		return "continent_geoname_id", nil
	case "event:city", "visit:city":
		return "city_geoname_id", nil
	case "visit:browser":
		return "browser", nil
	case "visit:browser_version":
		return "browser_version", nil
	case "visit:os":
		return "os", nil
	case "visit:os_version":
		return "os_version", nil
	case "visit:device":
		return "device", nil
	case "visit:screen_size":
		return "screen_size", nil
	// UTM 维度（event:/visit: 两种规范名映射到同一列，用于过滤）
	case "event:utm_source", "visit:utm_source":
		return "utm_source", nil
	case "event:utm_medium", "visit:utm_medium":
		return "utm_medium", nil
	case "event:utm_campaign", "visit:utm_campaign":
		return "utm_campaign", nil
	case "event:utm_content", "visit:utm_content":
		return "utm_content", nil
	case "event:utm_term", "visit:utm_term":
		return "utm_term", nil
	default:
		if strings.HasPrefix(name, "event:props:") {
			propName := strings.TrimPrefix(name, "event:props:")
			return fmt.Sprintf("meta.value[indexOf(meta.key, '%s')]", escapeCHString(propName)), nil
		}
		if strings.HasPrefix(name, "visit:entry_props:") {
			propName := strings.TrimPrefix(name, "visit:entry_props:")
			return fmt.Sprintf("entry_meta.value[indexOf(entry_meta.key, '%s')]", escapeCHString(propName)), nil
		}
		return name, nil
	}
}

func (wb *WhereBuilder) dbFieldVal(field string, val any) any {
	strVal, ok := val.(string)
	if !ok {
		return val
	}

	noRef := "Direct / None"
	notSet := "(not set)"

	switch field {
	case "source", "referrer", "utm_medium", "utm_source", "utm_campaign", "utm_content", "utm_term":
		if strVal == noRef {
			return ""
		}
	}

	if strVal == notSet {
		return ""
	}

	return val
}

// 目标事件过滤专用方法
// 将 goal 的 display_name 解析为对应的过滤条件（先查 PG，再构建纯 ClickHouse SQL）：
//   - event goal → events.name = goal.event_name
//   - pageview goal → events.name = 'pageview' AND events.pathname = goal.page_path
func (wb *WhereBuilder) addGoalFilter(filter *types.Filter) error {
	if filter.Operator != "is" && filter.Operator != "is_not" {
		return fmt.Errorf("goal filters support only is and is_not")
	}
	var names []string
	for _, value := range filter.Values {
		name, ok := value.(string)
		if !ok || name == "" {
			return fmt.Errorf("goal values must be non-empty strings")
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return fmt.Errorf("goal value cannot be empty")
	}
	parts := []string{}
	params := []any{}
	siteID, err := strconv.ParseInt(wb.siteID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid goal site ID: %w", err)
	}
	if db := globals.GetDB(); db != nil {
		goals, err := db.Client.Goal.Query().Where(goal.SiteID(siteID), goal.DisplayNameIn(names...)).All(context.Background())
		if err != nil {
			return fmt.Errorf("load goal filters: %w", err)
		}
		for _, g := range goals {
			sub := wb.newSubBuilder()
			if g.EventName != "" {
				sub.AddCondition("name = ?", g.EventName)
			} else if g.PagePath != "" {
				sub.AddCondition("name = 'pageview' AND pathname = ?", g.PagePath)
			} else {
				continue
			}
			sub.addGoalCustomProps(g.CustomProps)
			condition, args := sub.Build()
			parts = append(parts, "("+condition+")")
			params = append(params, args...)
		}
	}
	condition := "1 = 0"
	if len(parts) > 0 {
		condition = "(" + strings.Join(parts, " OR ") + ")"
	}
	if filter.Operator == "is_not" {
		condition = "NOT (" + condition + ")"
	}
	wb.AddCondition(condition, params...)
	return nil
}

// addGoalCustomProps 为 goal 的自定义属性添加 ClickHouse 过滤条件。
// custom_props 格式: {"plan": "pro", "source": "google"} →
//
//	has(meta.key, 'plan') AND meta.value[indexOf(meta.key, 'plan')] = 'pro'
//	AND has(meta.key, 'source') AND meta.value[indexOf(meta.key, 'source')] = 'google'
func (wb *WhereBuilder) addGoalCustomProps(props map[string]string) {
	if len(props) == 0 {
		return
	}
	for key, val := range props {
		// 转义反斜杠与单引号，防止 key/val 中的反斜杠或引号破坏 SQL 字符串字面量
		escapedKey := escapeCHString(key)
		escapedVal := escapeCHString(val)
		wb.AddCondition(fmt.Sprintf("has(meta.key, '%s')", escapedKey))
		wb.AddCondition(fmt.Sprintf("meta.value[indexOf(meta.key, '%s')] = '%s'", escapedKey, escapedVal))
	}
}

// 各种过滤条件的具体实现
func (wb *WhereBuilder) addIsFilter(field string, values any, modifiers map[string]string) error {
	list, ok := values.([]any)
	if !ok {
		return fmt.Errorf("invalid values type for 'is' filter: %T", values)
	}

	if len(list) == 0 {
		return fmt.Errorf("'is' filter requires at least one value")
	}

	placeholders := make([]string, len(list))
	params := make([]any, len(list))

	for i, v := range list {
		placeholders[i] = "?"
		params[i] = wb.dbFieldVal(field, v)
	}

	fieldExpr := field
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", field)
		for i, v := range params {
			if strVal, ok := v.(string); ok {
				params[i] = strings.ToLower(strVal)
			}
		}
	}
	if len(placeholders) == 1 {
		wb.AddCondition(fmt.Sprintf("%s = %s", fieldExpr, placeholders[0]), params...)
	} else {
		wb.AddCondition(fmt.Sprintf("%s IN (%s)", fieldExpr, strings.Join(placeholders, ",")), params...)
	}

	return nil
}

func (wb *WhereBuilder) addIsNotFilter(field string, values any, modifiers map[string]string) error {
	list, ok := values.([]any)
	if !ok {
		return fmt.Errorf("invalid values type for 'is_not' filter: %T", values)
	}

	if len(list) == 0 {
		return fmt.Errorf("'is_not' filter requires at least one value")
	}

	placeholders := make([]string, len(list))
	params := make([]any, len(list))

	for i, v := range list {
		placeholders[i] = "?"
		params[i] = wb.dbFieldVal(field, v)
	}

	fieldExpr := field
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", field)
		for i, v := range params {
			if strVal, ok := v.(string); ok {
				params[i] = strings.ToLower(strVal)
			}
		}
	}

	// 自定义属性特殊处理：不存在或值不在列表中
	if strings.HasPrefix(field, "event:props:") || strings.HasPrefix(field, "visit:entry_props:") {
		propName := strings.TrimPrefix(field, "event:props:")
		propName = strings.TrimPrefix(propName, "visit:entry_props:")
		column := "meta"
		if strings.HasPrefix(field, "visit:entry_props:") {
			column = "entry_meta"
		}
		condition := fmt.Sprintf("(not has(%s.key, ?) OR %s NOT IN (%s))", column, fieldExpr, strings.Join(placeholders, ","))
		allParams := append([]any{propName}, params...)
		wb.AddCondition(condition, allParams...)
	} else {
		wb.AddCondition(fmt.Sprintf("%s NOT IN (%s)", fieldExpr, strings.Join(placeholders, ",")), params...)
	}
	return nil
}

func (wb *WhereBuilder) addContainsFilter(field string, value any, modifiers map[string]string) error {
	var strVals []string

	// 检查value是否为[]any类型
	if interfaceSlice, ok := value.([]any); ok {
		// 将[]any转换为[]string
		strVals = make([]string, len(interfaceSlice))
		for i, v := range interfaceSlice {
			// 尝试将每个元素转换为string
			if str, ok := v.(string); ok {
				strVals[i] = str
			} else {
				// 如果转换失败，使用fmt.Sprintf来处理
				strVals[i] = fmt.Sprintf("%v", v)
			}
		}
	} else if str, ok := value.(string); ok {
		// 保持原有的处理逻辑
		strVals = []string{str}
	} else {
		return fmt.Errorf("invalid value type for 'contains' filter: %T", value)
	}

	fieldExpr := fmt.Sprintf("toString(%s)", field)
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", fieldExpr)
		// 对所有字符串进行小写转换
		for i, strVal := range strVals {
			strVals[i] = strings.ToLower(strVal)
		}
	}

	// 将字符串切片作为数组参数传递给multiSearchAny
	wb.AddCondition(fmt.Sprintf("multiSearchAny(%s, ?)", fieldExpr), strVals)
	return nil
}

func (wb *WhereBuilder) addContainsNotFilter(field string, value any, modifiers map[string]string) error {
	var strVals []string

	// 检查value是否为[]any类型
	if interfaceSlice, ok := value.([]any); ok {
		// 将[]any转换为[]string
		strVals = make([]string, len(interfaceSlice))
		for i, v := range interfaceSlice {
			// 尝试将每个元素转换为string
			if str, ok := v.(string); ok {
				strVals[i] = str
			} else {
				// 如果转换失败，使用fmt.Sprintf来处理
				strVals[i] = fmt.Sprintf("%v", v)
			}
		}
	} else if str, ok := value.(string); ok {
		// 保持原有的处理逻辑
		strVals = []string{str}
	} else {
		return fmt.Errorf("invalid value type for 'contains_not' filter: %T", value)
	}

	fieldExpr := fmt.Sprintf("toString(%s)", field)
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", fieldExpr)
		// 对所有字符串进行小写转换
		for i, strVal := range strVals {
			strVals[i] = strings.ToLower(strVal)
		}
	}

	// 将字符串切片作为数组参数传递给multiSearchAny

	wb.AddCondition(fmt.Sprintf("NOT multiSearchAny(%s, ?)", fieldExpr), strVals)
	return nil
}

// 添加多匹配条件处理
func (wb *WhereBuilder) addMatchesFilter(fieldExpr string, patterns any, modifiers map[string]string) error {
	patternList, ok := toStringSlice(patterns)
	if !ok {
		return fmt.Errorf("invalid patterns type for 'matches' filter: %T", patterns)
	}

	// 处理大小写敏感
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", fieldExpr)
		for i, p := range patternList {
			patternList[i] = strings.ToLower(p)
		}
	}

	// 添加多模式匹配条件
	wb.AddCondition(fmt.Sprintf("multiMatchAny(%s, ?)", fieldExpr), patternList)
	return nil
}

func (wb *WhereBuilder) addMatchesNotFilter(field string, patterns any, modifiers map[string]string) error {
	patternList, ok := toStringSlice(patterns)
	if !ok {
		return fmt.Errorf("invalid patterns type for 'matches_not' filter: %T", patterns)
	}

	fieldExpr := field
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", field)
		for i, p := range patternList {
			patternList[i] = strings.ToLower(p)
		}
	}

	wb.AddCondition(fmt.Sprintf("NOT multiMatchAny(%s, ?)", fieldExpr), patternList)
	return nil
}

func (wb *WhereBuilder) addMatchesWildcardFilter(field string, patterns any, modifiers map[string]string) error {
	patternList, ok := toStringSlice(patterns)
	if !ok {
		return fmt.Errorf("invalid patterns type for 'matches_wildcard' filter: %T", patterns)
	}

	regexPatterns := make([]string, len(patternList))
	for i, p := range patternList {
		if modifiers["case_sensitive"] == "false" {
			p = strings.ToLower(p)
		}
		regexPatterns[i] = wildcardToRegex(p)
	}

	fieldExpr := field
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", field)
	}

	wb.AddCondition(fmt.Sprintf("multiMatchAny(%s, ?)", fieldExpr), regexPatterns)
	return nil
}

func (wb *WhereBuilder) addMatchesWildcardNotFilter(field string, patterns any, modifiers map[string]string) error {
	patternList, ok := toStringSlice(patterns)
	if !ok {
		return fmt.Errorf("invalid patterns type for 'matches_wildcard_not' filter: %T", patterns)
	}

	regexPatterns := make([]string, len(patternList))
	for i, p := range patternList {
		if modifiers["case_sensitive"] == "false" {
			p = strings.ToLower(p)
		}
		regexPatterns[i] = wildcardToRegex(p)
	}

	fieldExpr := field
	if val, ok := modifiers["case_sensitive"]; ok && val == "false" {
		fieldExpr = fmt.Sprintf("lower(%s)", field)
	}

	wb.AddCondition(fmt.Sprintf("NOT multiMatchAny(%s, ?)", fieldExpr), regexPatterns)
	return nil
}

func toStringSlice(value any) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, str)
		}
		return result, true
	case string:
		return []string{v}, true
	default:
		return nil, false
	}
}

// wildcardToRegex converts a wildcard pattern to a regex pattern.
// Interior * and ? are converted to .* and . before QuoteMeta escaping so that
// only the non-wildcard segments are escaped.
func wildcardToRegex(wildcard string) string {
	// Split on wildcard characters, escape each literal segment, then rejoin.
	var buf strings.Builder
	buf.WriteString("^")
	for len(wildcard) > 0 {
		idx := strings.IndexAny(wildcard, "*?")
		if idx < 0 {
			buf.WriteString(regexp.QuoteMeta(wildcard))
			break
		}
		buf.WriteString(regexp.QuoteMeta(wildcard[:idx]))
		if wildcard[idx] == '*' {
			buf.WriteString(".*")
		} else {
			buf.WriteString(".")
		}
		wildcard = wildcard[idx+1:]
	}
	buf.WriteString("$")
	return buf.String()
}

// escapeCHString Escape ClickHouse single-quoted string字面量中的特殊字符，
// Prevent SQL injection。
func escapeCHString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

// safeAlias 将任意属性名转换为安全的 SQL 别名标识符，
// 仅保留字母、数字和下划线，其余字符替换为下划线。
func safeAlias(s string) string {
	var buf strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('_')
		}
	}
	alias := buf.String()
	if alias == "" {
		return "prop"
	}
	// 别名不能以数字开头
	if alias[0] >= '0' && alias[0] <= '9' {
		alias = "p_" + alias
	}
	return alias
}
