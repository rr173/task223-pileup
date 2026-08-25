// Package deconv 提供辐射探测器脉冲堆积的受约束解卷积。
//
// 核心算法为贪心匹配追踪（matching pursuit）：以锁定的参考脉冲为核，
// 反复在残差上做互相关、定位最强峰、减去对应幅度的参考脉冲，直至残差
// 降到噪声底以下。约束包括：恢复幅度非负、相邻脉冲间隔不小于死区时间、
// 残差占比不超过上限（超限则判定为不可分离）。
package deconv

import "math"

// RecoveredPulse 是解卷积从堆积波形中恢复出的单个脉冲。
type RecoveredPulse struct {
	Position  int     // 样本索引（相对窗口起点）
	Amplitude float64 // 恢复幅度（相对参考脉冲归一化幅度）
}

// Result 是一次解卷积的输出。
type Result struct {
	Pulses     []RecoveredPulse // 恢复出的脉冲（按位置升序）
	Residual   float64          // 残差能量占比（0~1）
	Iterations int              // 实际迭代次数
}

// Deconvolver 受约束匹配追踪解卷积器。
type Deconvolver struct {
	MaxIterations int     // 最大迭代次数
	Threshold     float64 // 停止阈值：残差峰幅度低于此值即停止
}

// NewDeconvolver 构造解卷积器（默认最多 64 次迭代）。
func NewDeconvolver() *Deconvolver {
	return &Deconvolver{MaxIterations: 64, Threshold: 0.02}
}

// Deconvolve 在去基线波形上执行受约束解卷积。
//
// 参数：
//   - wave：去基线后的窗口波形（长度须大于 ref）。
//   - ref：归一化参考脉冲（峰值应为 1.0）。
//   - minSeparation：相邻恢复脉冲的最小间隔（样本），用于抑制在同一脉冲
//     尖峰上的重复检出，取值应接近脉冲可分辨宽度而非死区时间。
//   - noiseFloor：噪声底，用于自适应停止阈值。
func (d *Deconvolver) Deconvolve(wave, ref []float64, minSeparation int, noiseFloor float64) Result {
	if len(wave) < len(ref) || len(ref) == 0 {
		return Result{Residual: 1.0}
	}
	residual := make([]float64, len(wave))
	copy(residual, wave)

	kernelEnergy := 0.0
	for _, v := range ref {
		kernelEnergy += v * v
	}
	if kernelEnergy <= 0 {
		return Result{Residual: 1.0}
	}

	stopThreshold := d.Threshold
	if noiseFloor > stopThreshold {
		stopThreshold = noiseFloor
	}

	if minSeparation < 1 {
		minSeparation = 1
	}

	// 参考脉冲峰值在 ref 中的位置（ref 不必以 0 为峰值起点）。
	refCenter := 0
	for i, v := range ref {
		if v > ref[refCenter] {
			refCenter = i
		}
	}

	var pulses []RecoveredPulse
	iter := 0
	for iter < d.MaxIterations {
		corr := crossCorrelate(residual, ref)
		// 最小间隔约束：屏蔽与已恢复脉冲过近的位置，避免在同一脉冲尖峰
		// 上重复检出。corr 的下标 i 对应「ref 起点在 i」，脉冲中心在 i+refCenter。
		for _, p := range pulses {
			for j := 0; j < len(corr); j++ {
				if absInt((j+refCenter)-p.Position) < minSeparation {
					corr[j] = 0
				}
			}
		}
		pos, val := argMax(corr)
		if val < stopThreshold {
			break
		}
		amp := val / kernelEnergy
		pulses = append(pulses, RecoveredPulse{Position: pos + refCenter, Amplitude: amp})

		// 从残差中减去该脉冲贡献。
		for k := 0; k < len(ref); k++ {
			idx := pos + k
			if idx >= 0 && idx < len(residual) {
				residual[idx] -= amp * ref[k]
			}
		}
		iter++
	}

	// 按位置升序排序。
	sortPulses(pulses)

	residualEnergy := energy(residual)
	inputEnergy := energy(wave)
	ratio := 0.0
	if inputEnergy > 0 {
		ratio = math.Sqrt(residualEnergy / inputEnergy)
	}
	return Result{Pulses: pulses, Residual: ratio, Iterations: iter}
}

// crossCorrelate 计算 signal 与 kernel 的互相关：
// corr[i] = Σ_k signal[i+k] * kernel[k]，结果长度为 len(signal)-len(kernel)+1。
func crossCorrelate(signal, kernel []float64) []float64 {
	outLen := len(signal) - len(kernel) + 1
	if outLen <= 0 {
		return nil
	}
	out := make([]float64, outLen)
	for i := 0; i < outLen; i++ {
		var s float64
		for k := 0; k < len(kernel); k++ {
			s += signal[i+k] * kernel[k]
		}
		out[i] = s
	}
	return out
}

// argMax 返回切片最大值及其下标（空切片返回 0,0）。
func argMax(v []float64) (int, float64) {
	if len(v) == 0 {
		return 0, 0
	}
	bestIdx, bestVal := 0, v[0]
	for i := 1; i < len(v); i++ {
		if v[i] > bestVal {
			bestVal = v[i]
			bestIdx = i
		}
	}
	return bestIdx, bestVal
}

// energy 返回切片能量（平方和）。
func energy(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return s
}

// absInt 返回整数绝对值。
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// sortPulses 按位置升序原地排序。
func sortPulses(p []RecoveredPulse) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && p[j].Position < p[j-1].Position; j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}

// sortPulsesByAmpDesc 按幅度降序原地排序（幅度相同者保持稳定相对顺序）。
func sortPulsesByAmpDesc(p []RecoveredPulse) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && p[j].Amplitude > p[j-1].Amplitude; j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}
