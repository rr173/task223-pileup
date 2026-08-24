package counting

import "math"

// DeadTimeCorrect 非扩展（non-paralyzable）死区校正：
//
//	n = m / (1 - m·τ)
//
// 其中 m 为观测计数率（counts/s），τ 为死区时间（秒），n 为真实计数率。
// 当 m·τ 趋近 1 时探测器接近饱和，返回正无穷表示计数不可恢复。
func DeadTimeCorrect(observedRate, deadTimeSeconds float64) float64 {
	if observedRate <= 0 || deadTimeSeconds <= 0 {
		return observedRate
	}
	denom := 1 - observedRate*deadTimeSeconds
	if denom <= 1e-9 {
		return math.Inf(1)
	}
	return observedRate / denom
}

// DeadTimeLossFraction 返回因死区导致的计数损失占比：
// loss = m·τ（观测计数率与死区时间的乘积）。
func DeadTimeLossFraction(observedRate, deadTimeSeconds float64) float64 {
	if observedRate <= 0 || deadTimeSeconds <= 0 {
		return 0
	}
	return observedRate * deadTimeSeconds
}

// RateToCounts 把计数率换算为某观察时长内的计数量。
func RateToCounts(rate float64, observationSeconds float64) int {
	if rate <= 0 || observationSeconds <= 0 {
		return 0
	}
	return int(math.Round(rate * observationSeconds))
}
