package stats

import (
	"errors"
	"time"

	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// Interval 定义时间间隔类型
type Interval string

// 支持的时间间隔
const (
	IntervalMinute  Interval = "minute"
	IntervalHourly  Interval = "hourly"
	IntervalDaily   Interval = "daily"
	IntervalWeekly  Interval = "weekly"
	IntervalMonthly Interval = "monthly"
	IntervalYearly  Interval = "yearly"
)

// ParseInterval 将字符串解析为Interval类型
func ParseInterval(interval string) (Interval, error) {
	switch interval {
	case string(IntervalMinute):
		return IntervalMinute, nil
	case string(IntervalHourly):
		return IntervalHourly, nil
	case string(IntervalDaily):
		return IntervalDaily, nil
	case string(IntervalWeekly):
		return IntervalWeekly, nil
	case string(IntervalMonthly):
		return IntervalMonthly, nil
	case string(IntervalYearly):
		return IntervalYearly, nil
	case "":
		return IntervalDaily, nil
	default:
		return "", errors.New("invalid interval")
	}
}

// DefaultIntervalForPeriod 根据周期类型返回默认的时间间隔。
// day/yesterday 默认使用 hourly，realtime 默认使用 minute，其他周期默认使用 daily。
func DefaultIntervalForPeriod(period string) string {
	switch period {
	case "day", "yesterday":
		return "hourly"
	case "realtime":
		return "minute"
	default:
		return "daily"
	}
}

// StartOfInterval 计算给定时间所在间隔的起始时间
func StartOfInterval(t time.Time, interval Interval) time.Time {
	switch interval {
	case IntervalDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case IntervalWeekly:
		// 周一开始
		weekday := t.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		daysToSubtract := int(weekday - 1)
		return time.Date(t.Year(), t.Month(), t.Day()-daysToSubtract, 0, 0, 0, 0, t.Location())
	case IntervalMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case IntervalYearly:
		return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location())
	default:
		return t
	}
}

// EndOfInterval 计算给定时间所在间隔的结束时间
func EndOfInterval(t time.Time, interval Interval) time.Time {
	switch interval {
	case IntervalDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
	case IntervalWeekly:
		// 周日结束
		weekday := t.Weekday()
		if weekday == time.Sunday {
			return EndOfInterval(t, IntervalDaily)
		}
		daysToAdd := int(7 - weekday)
		return EndOfInterval(t.AddDate(0, 0, daysToAdd), IntervalDaily)
	case IntervalMonthly:
		return time.Date(t.Year(), t.Month()+1, 0, 23, 59, 59, 999999999, t.Location())
	case IntervalYearly:
		return time.Date(t.Year()+1, time.January, 0, 23, 59, 59, 999999999, t.Location())
	default:
		return t
	}
}

// GenerateTimeRanges 生成指定时间范围内所有间隔的起止时间
func GenerateTimeRanges(start, end time.Time, interval Interval) ([]types.TimeRange, error) {
	if start.After(end) {
		return nil, errors.New("start time must be before end time")
	}

	var ranges []types.TimeRange
	currentStart := StartOfInterval(start, interval)

	for {
		currentEnd := EndOfInterval(currentStart, interval)
		if currentStart.After(end) {
			break
		}

		// 确保不超过结束时间
		if currentEnd.After(end) {
			currentEnd = end
		}

		ranges = append(ranges, types.TimeRange{
			Start: currentStart,
			End:   currentEnd,
		})

		// 移动到下一个间隔
		switch interval {
		case IntervalMinute:
			currentStart = currentStart.Add(time.Minute)
		case IntervalHourly:
			currentStart = currentStart.Add(time.Hour)
		case IntervalDaily:
			currentStart = currentStart.AddDate(0, 0, 1)
		case IntervalWeekly:
			currentStart = currentStart.AddDate(0, 0, 7)
		case IntervalMonthly:
			currentStart = currentStart.AddDate(0, 1, 0)
		case IntervalYearly:
			currentStart = currentStart.AddDate(1, 0, 0)
		default:
			return nil, errors.New("invalid interval")
		}
	}

	return ranges, nil
}
