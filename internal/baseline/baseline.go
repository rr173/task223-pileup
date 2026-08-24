// Package baseline 提供辐射探测器波形的基线估计与漂移检测。
// 基线估计用滑动中位数抵抗脉冲尖峰的干扰，漂移检测用线性回归捕捉
// 基线随时间的缓慢移动，二者共同决定波形是否落在可恢复区间内。
package baseline

import (
	"math"
	"sort"
)

// Result 是一次基线估计的输出。
type Result struct {
	Level       float64 // 基线水平（归一化）
	DriftSlope  float64 // 漂移斜率（每窗口相对变化）
	NoiseFloor  float64 // 噪声底（残差均方根）
	WindowCount int     // 参与估计的窗口数
}

// DriftExceeded reports whether the fitted run-level drift violates the
// detector's slope budget.
func (r Result) DriftExceeded(d *DriftDetector) bool {
	return d != nil && d.SlopeExceeded(r.DriftSlope)
}

// Estimator 从多个波形窗口估计基线参数。
type Estimator struct{}

// NewEstimator 构造基线估计器。
func NewEstimator() *Estimator { return &Estimator{} }

// Estimate 从一组窗口波形估计基线水平、漂移斜率与噪声底。
//
// 算法：对每个窗口取样本中位数作为该窗口基线（中位数对脉冲尖峰鲁棒），
// 所有窗口基线取中位数得 Level；对「窗口基线 vs 窗口序号」做最小二乘
// 线性回归得 DriftSlope；用窗口基线相对回归直线的残差均方根作为 NoiseFloor。
func (e *Estimator) Estimate(windows [][]float64) Result {
	if len(windows) == 0 {
		return Result{}
	}
	perWindow := make([]float64, 0, len(windows))
	for _, w := range windows {
		if len(w) == 0 {
			continue
		}
		perWindow = append(perWindow, medianFloat(w))
	}
	if len(perWindow) == 0 {
		return Result{}
	}
	level := medianFloat(perWindow)

	// 最小二乘线性回归：y = a*x + b（x 为窗口序号）。
	n := float64(len(perWindow))
	var sumX, sumY, sumXY, sumXX float64
	for i, y := range perWindow {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	slope := 0.0
	denom := n*sumXX - sumX*sumX
	if math.Abs(denom) > 1e-12 {
		slope = (n*sumXY - sumX*sumY) / denom
	}

	// 残差 RMS。
	intercept := (sumY - slope*sumX) / n
	var sumSq float64
	for i, y := range perWindow {
		pred := slope*float64(i) + intercept
		sumSq += (y - pred) * (y - pred)
	}
	noiseFloor := math.Sqrt(sumSq / n)

	return Result{Level: level, DriftSlope: slope, NoiseFloor: noiseFloor, WindowCount: len(perWindow)}
}

// medianFloat 返回浮点切片的中位数（会复制排序，不修改入参）。
func medianFloat(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := make([]float64, len(v))
	copy(c, v)
	sort.Float64s(c)
	mid := len(c) / 2
	if len(c)%2 == 1 {
		return c[mid]
	}
	return (c[mid-1] + c[mid]) / 2
}
