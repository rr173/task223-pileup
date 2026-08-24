package snapshot

import (
	"fmt"
	"math"
)

// FormatRate 格式化计数率：按数量级选择合适的单位（cps / kcps / Mcps）。
func FormatRate(rate float64) string {
	if math.IsInf(rate, 1) {
		return "saturated"
	}
	switch {
	case rate >= 1e6:
		return fmt.Sprintf("%.3f Mcps", rate/1e6)
	case rate >= 1e3:
		return fmt.Sprintf("%.3f kcps", rate/1e3)
	default:
		return fmt.Sprintf("%.3f cps", rate)
	}
}

// FormatDeadTime 格式化死区占比为百分数字符串。
func FormatDeadTime(frac float64) string {
	return fmt.Sprintf("%.2f%%", frac*100)
}

// FormatDurationNs 把纳秒时长渲染为可读字符串（ns/us/ms/s）。
func FormatDurationNs(ns int64) string {
	switch {
	case ns >= 1e9:
		return fmt.Sprintf("%.3f s", float64(ns)/1e9)
	case ns >= 1e6:
		return fmt.Sprintf("%.3f ms", float64(ns)/1e6)
	case ns >= 1e3:
		return fmt.Sprintf("%.3f us", float64(ns)/1e3)
	default:
		return fmt.Sprintf("%d ns", ns)
	}
}
