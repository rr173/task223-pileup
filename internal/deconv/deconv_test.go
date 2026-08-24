package deconv

import (
	"math"
	"testing"
)

// 构造一个高斯参考脉冲（归一化）。
func gaussianPulse(n int, center, sigma float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		x := float64(i) - center
		s[i] = math.Exp(-(x * x) / (2 * sigma * sigma))
	}
	return s
}

// 把脉冲叠加到波形上。
func addPulse(wave []float64, center, amp, sigma float64) {
	for i := range wave {
		x := float64(i) - center
		wave[i] += amp * math.Exp(-(x*x)/(2*sigma*sigma))
	}
}

// TestDeconvolveSeparatesTwoPulses 验证解卷积能从堆积波形中恢复两个脉冲。
func TestDeconvolveSeparatesTwoPulses(t *testing.T) {
	// 参考脉冲为短片段（中心在 index 10）。
	ref := gaussianPulse(21, 10, 4.0)

	// 两个脉冲间距 10 样本，小于死区（如 20）→ 堆积，但大于可分辨宽度。
	wave := make([]float64, 200)
	addPulse(wave, 100, 0.8, 4.0)
	addPulse(wave, 110, 0.6, 4.0)

	d := NewDeconvolver()
	result := d.Deconvolve(wave, ref, 2, 0.01)

	c := DefaultConstraints()
	filtered := c.Filter(result.Pulses)
	if len(filtered) < 2 {
		t.Fatalf("expected >= 2 recovered pulses, got %d", len(filtered))
	}
	// 重叠脉冲的匹配追踪存在系统性位置偏差，但两个恢复脉冲应分别落在
	// 100 与 110 附近（容差 ±8），且间距接近真实间距 10。
	if math.Abs(float64(filtered[0].Position-100)) > 8 {
		t.Errorf("first pulse position %d too far from 100", filtered[0].Position)
	}
	if math.Abs(float64(filtered[1].Position-110)) > 8 {
		t.Errorf("second pulse position %d too far from 110", filtered[1].Position)
	}
	gap := filtered[1].Position - filtered[0].Position
	if gap < 8 || gap > 14 {
		t.Errorf("recovered gap %d out of expected range [8,14]", gap)
	}
	if result.Residual > c.MaxResidual {
		t.Errorf("residual %f exceeds max %f", result.Residual, c.MaxResidual)
	}
}

// TestDeconvolveIsolatedPulse 验证孤立脉冲被直接恢复。
func TestDeconvolveIsolatedPulse(t *testing.T) {
	ref := gaussianPulse(21, 10, 4.0)
	wave := make([]float64, 200)
	addPulse(wave, 100, 0.8, 4.0)

	d := NewDeconvolver()
	result := d.Deconvolve(wave, ref, 2, 0.01)

	filtered := DefaultConstraints().Filter(result.Pulses)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 recovered pulse, got %d", len(filtered))
	}
	if math.Abs(filtered[0].Amplitude-0.8) > 0.15 {
		t.Errorf("recovered amplitude %f too far from 0.8", filtered[0].Amplitude)
	}
}

// TestNonNegative 验证约束校验负幅度。
func TestNonNegative(t *testing.T) {
	if !NonNegative([]RecoveredPulse{{Position: 1, Amplitude: 0.5}, {Position: 2, Amplitude: 0.2}}) {
		t.Error("expected non-negative pulses to pass")
	}
	if NonNegative([]RecoveredPulse{{Position: 1, Amplitude: -0.1}}) {
		t.Error("expected negative pulse to fail")
	}
}

// TestHalfWidthSamples 验证半高宽计算。
func TestHalfWidthSamples(t *testing.T) {
	shape := gaussianPulse(101, 50, 10.0)
	w := HalfWidthSamples(shape)
	if w <= 0 {
		t.Fatalf("half width should be positive, got %d", w)
	}
	// 高斯 σ=10 的半高宽约 2.355*σ ≈ 23.6，允许一定容差。
	if w < 18 || w > 30 {
		t.Errorf("half width %d out of expected range", w)
	}
}
