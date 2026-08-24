package detector

import "testing"

// TestPeakDetectorFindsLocalMaxima 验证峰值检测定位局部最大值。
func TestPeakDetectorFindsLocalMaxima(t *testing.T) {
	wave := []float64{0, 0, 0.1, 0.4, 0.9, 0.4, 0.1, 0, 0, 0.1, 0.5, 0.1, 0}
	d := NewPeakDetector(0.05, 2)
	peaks := d.Detect(wave)
	if len(peaks) != 2 {
		t.Fatalf("expected 2 peaks, got %d", len(peaks))
	}
	if peaks[0].Position != 4 {
		t.Errorf("first peak position = %d, want 4", peaks[0].Position)
	}
	if peaks[1].Position != 10 {
		t.Errorf("second peak position = %d, want 10", peaks[1].Position)
	}
}

// TestPeakDetectorIgnoresBelowThreshold 验证低于阈值的峰被忽略。
func TestPeakDetectorIgnoresBelowThreshold(t *testing.T) {
	wave := []float64{0, 0, 0.02, 0.03, 0.02, 0}
	d := NewPeakDetector(0.05, 2)
	if peaks := d.Detect(wave); len(peaks) != 0 {
		t.Fatalf("expected 0 peaks, got %d", len(peaks))
	}
}

// TestPileUpDetectorGroups 验证堆积识别把靠得过近的峰归并为一组。
func TestPileUpDetectorGroups(t *testing.T) {
	peaks := []Peak{
		{Position: 100, Amplitude: 0.8},
		{Position: 108, Amplitude: 0.6}, // 与 100 间距 8 < 10 → 同组
		{Position: 200, Amplitude: 0.9}, // 与 108 间距 92 → 新组
	}
	d := NewPileUpDetector(10)
	groups := d.Group(peaks)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if !groups[0].IsPiled() {
		t.Error("first group should be piled")
	}
	if groups[1].IsPiled() {
		t.Error("second group should be isolated")
	}
}
