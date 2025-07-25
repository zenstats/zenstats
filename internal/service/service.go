package service

import "github.com/dromara/carbon/v2"

// 调整开始时间到间隔起始点
func adjustStartTimeToInterval(start *carbon.Carbon, interval string, intervalSeconds int) *carbon.Carbon {
	if start == nil {
		return nil
	}

	adjusted := start.Copy()
	if adjusted == nil {
		return nil
	}

	switch interval {
	case "5 MINUTE", "10 MINUTE":
		intervalMinute := intervalSeconds / 60
		currentMinute := adjusted.Minute()
		adjustment := currentMinute % intervalMinute
		return adjusted.SubMinutes(adjustment)
	case "1 HOUR":
		return adjusted.StartOfHour()
	case "1 DAY":
		return adjusted.StartOfDay()
	default:
		return adjusted
	}
}

// 根据间隔获取时间格式
func getTimeFormatByInterval(interval string) string {
	switch interval {
	case "5 MINUTE", "10 MINUTE", "1 HOUR":
		return "Y-m-d H:i"
	case "1 DAY":
		return "Y-m-d"
	default:
		return "Y-m-d H:i"
	}
}

// getIntervalSeconds 将间隔字符串转换为秒数
func getIntervalSeconds(interval string) int {
	switch interval {
	case "1 MINUTE":
		return 60
	case "5 MINUTE":
		return 5 * 60
	case "10 MINUTE":
		return 10 * 60
	case "1 HOUR":
		return 60 * 60
	case "1 DAY":
		return 24 * 60 * 60
	case "1 WEEK":
		return 24 * 60 * 60 * 7
	default:
		return 0
	}
}
