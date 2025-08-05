package stats

import (
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

type ComparisonMode string

const (
	PreviousPeriod ComparisonMode = "previous_period"
	YearOverYear   ComparisonMode = "year_over_year"
	Custom         ComparisonMode = "custom"
)

type ComparisonOptions struct {
	Mode           ComparisonMode
	DateRange      *types.TimeRange
	MatchDayOfWeek bool
}

func GetComparisonDateRange(source types.TimeRange, opts ComparisonOptions) types.TimeRange {
	switch opts.Mode {
	case YearOverYear:
		start := source.Start.AddDate(-1, 0, 0)
		end := source.End.AddDate(-1, 0, 0)
		if opts.MatchDayOfWeek {
			start = shiftToWeekday(start, source.Start.Weekday())
			end = start.Add(source.End.Sub(source.Start))
		}
		return types.TimeRange{Start: start, End: end}
	case PreviousPeriod:
		diff := source.End.Sub(source.Start)
		start := source.Start.Add(-diff)
		end := source.End.Add(-diff)
		if opts.MatchDayOfWeek {
			start = shiftToWeekday(start, source.Start.Weekday())
			end = start.Add(diff)
		}
		return types.TimeRange{Start: start, End: end}
	case Custom:
		if opts.DateRange != nil {
			return *opts.DateRange
		}
	}
	return source
}

func shiftToWeekday(date time.Time, weekday time.Weekday) time.Time {
	for date.Weekday() != weekday {
		date = date.AddDate(0, 0, 1)
	}
	return date
}

// 构建对比查询（伪代码，具体实现需结合你的 Query 结构）
func GetComparisonQuery(sourceQuery *types.Query, opts ComparisonOptions) *types.Query {
	compRange := GetComparisonDateRange(sourceQuery.UTCTimeRange, opts)
	compQuery := sourceQuery
	compQuery.ComparisonUTCTimeRange = &compRange

	return compQuery
}
