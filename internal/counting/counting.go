// Package counting 提供辐射计数的汇总与真实计数率校正。
// 观测计数率受脉冲堆积与死区影响偏低，计数汇总在扣除不可恢复区后
// 计算有效观察时间，再由真实计数率校正模型还原物理计数率。
package counting

// Summary 是一次计数汇总的结果。
type Summary struct {
	TotalCounts            int     // 总计数（分离确认 + 恢复）
	RecoveredCounts        int     // 解卷积恢复的堆积计数
	UnresolvedCounts       int     // 不可分离计数
	ObservedCountRate      float64 // 观测计数率（counts/s）
	TrueCountRate          float64 // 真实计数率（死区校正后，counts/s）
	EffectiveObservationNs int64   // 有效观察时间（扣除死区）
	DeadTimeFraction       float64 // 死区占比（0~1）
	UnrecoverableZones     int     // 不可恢复区数
}

// Aggregate 汇总计数：以分离确认脉冲为观测计数，扣除死区得到有效观察时间，
// 再计算观测计数率与真实计数率。
//
//   - separated：已分离/确认的脉冲数（观测计数）。
//   - unresolved：不可分离脉冲数。
//   - recovered：解卷积从堆积中恢复的脉冲数。
//   - deadTimeSeconds：探测器死区时间（秒）。
//   - totalObservationNs：窗口总观察时间（纳秒）。
//   - deadZoneNs：不可恢复区总时长（纳秒）。
//   - zones：不可恢复区数。
func Aggregate(separated, unresolved, recovered int, deadTimeSeconds float64,
	totalObservationNs, deadZoneNs int64, zones int) Summary {

	effective := totalObservationNs - deadZoneNs
	if effective < 0 {
		effective = 0
	}
	var observed float64
	if effective > 0 {
		observed = float64(separated) / (float64(effective) / 1e9)
	}
	trueRate := DeadTimeCorrect(observed, deadTimeSeconds)

	deadFraction := 0.0
	if totalObservationNs > 0 {
		deadFraction = float64(deadZoneNs) / float64(totalObservationNs)
	}

	return Summary{
		TotalCounts:            separated + unresolved,
		RecoveredCounts:        recovered,
		UnresolvedCounts:       unresolved,
		ObservedCountRate:      observed,
		TrueCountRate:          trueRate,
		EffectiveObservationNs: effective,
		DeadTimeFraction:       deadFraction,
		UnrecoverableZones:     zones,
	}
}
