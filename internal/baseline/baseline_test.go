package baseline

import (
	"math"
	"testing"
)

func TestEstimateIsRobustToPulseSpikesAndTracksDrift(t *testing.T) {
	windows := [][]float64{
		{0.10, 0.10, 0.10, 0.10, 1.00},
		{0.12, 0.12, 0.12, 0.12, 1.20},
		{0.14, 0.14, 0.14, 0.14, 1.40},
	}

	result := NewEstimator().Estimate(windows)
	if result.WindowCount != 3 {
		t.Fatalf("window count = %d, want 3", result.WindowCount)
	}
	if math.Abs(result.Level-0.12) > 1e-9 {
		t.Fatalf("level = %f, want 0.12", result.Level)
	}
	if math.Abs(result.DriftSlope-0.02) > 1e-9 {
		t.Fatalf("drift slope = %f, want 0.02", result.DriftSlope)
	}
	if result.NoiseFloor > 1e-9 {
		t.Fatalf("noise floor = %f, want zero for linear drift", result.NoiseFloor)
	}
}

func TestEstimateSkipsEmptyWindows(t *testing.T) {
	result := NewEstimator().Estimate([][]float64{{}, {0.3, 0.3}, {}})
	if result.WindowCount != 1 || result.Level != 0.3 {
		t.Fatalf("empty windows should be skipped, got %+v", result)
	}
}
