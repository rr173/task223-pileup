package counting

import (
	"math"
	"testing"
)

// TestDeadTimeCorrect 验证非扩展死区校正公式 n = m / (1 - m·τ)。
func TestDeadTimeCorrect(t *testing.T) {
	// 观测 100 cps，死区 2 ms → n = 100 / (1 - 0.2) = 125 cps。
	n := DeadTimeCorrect(100, 0.002)
	if math.Abs(n-125) > 0.01 {
		t.Errorf("expected 125 cps, got %f", n)
	}
}

// TestDeadTimeCorrectSaturation 验证接近饱和时返回正无穷。
func TestDeadTimeCorrectSaturation(t *testing.T) {
	// 观测 500 cps，死区 2 ms → m·τ = 1.0，探测器饱和。
	n := DeadTimeCorrect(500, 0.002)
	if !math.IsInf(n, 1) {
		t.Errorf("expected +Inf, got %f", n)
	}
}

// TestDeadTimeCorrectZero 验证零观测或零死区时返回原值。
func TestDeadTimeCorrectZero(t *testing.T) {
	if DeadTimeCorrect(0, 0.002) != 0 {
		t.Error("zero observed rate should stay zero")
	}
	if DeadTimeCorrect(100, 0) != 100 {
		t.Error("zero dead time should return observed rate")
	}
}

// TestAggregate 验证计数汇总的有效观察时间与死区占比。
func TestAggregate(t *testing.T) {
	sum := Aggregate(100, 10, 90, 0.002, 1_000_000_000, 200_000_000, 3)
	if sum.TotalCounts != 110 {
		t.Errorf("total counts = %d, want 110", sum.TotalCounts)
	}
	if sum.EffectiveObservationNs != 800_000_000 {
		t.Errorf("effective observation = %d, want 800000000", sum.EffectiveObservationNs)
	}
	// 死区占比 = 200ms / 1000ms = 0.2。
	if math.Abs(sum.DeadTimeFraction-0.2) > 0.001 {
		t.Errorf("dead time fraction = %f, want 0.2", sum.DeadTimeFraction)
	}
	if sum.ObservedCountRate <= 0 {
		t.Errorf("observed count rate should be positive")
	}
}
