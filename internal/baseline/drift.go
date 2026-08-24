package baseline

import "math"

// DriftWindow 描述一个窗口内基线的漂移量（相对全局基线水平）。
type DriftWindow struct {
	Index      int     // 窗口序号
	WindowBase float64 // 该窗口的基线估计
}

// DriftDetector 依据基线漂移判定窗口是否落在不可恢复区。
type DriftDetector struct {
	// MaxDrift 允许的最大基线漂移（相对基线水平的绝对偏差）。
	MaxDrift float64
	// MaxSlope 允许的最大漂移斜率（每窗口）。
	MaxSlope float64
}

// NewDriftDetector 构造漂移检测器（默认阈值：漂移 0.25、斜率 0.02）。
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{MaxDrift: 0.25, MaxSlope: 0.02}
}

// Classify 判定单个窗口是否因基线漂移而不可恢复。
func (d *DriftDetector) Classify(level float64, w DriftWindow) bool {
	if math.Abs(w.WindowBase-level) > d.MaxDrift {
		return true
	}
	return false
}

// ClassifyWithSlope combines a window-local offset check with the run-level
// slope budget. A slowly accumulating drift can stay within the local limit
// while still making the run unsuitable for recovery.
func (d *DriftDetector) ClassifyWithSlope(level float64, w DriftWindow, slope float64) bool {
	return d.Classify(level, w) || d.SlopeExceeded(slope)
}

// SlopeExceeded 判定整体漂移斜率是否超阈值。
func (d *DriftDetector) SlopeExceeded(slope float64) bool {
	return math.Abs(slope) > d.MaxSlope
}
