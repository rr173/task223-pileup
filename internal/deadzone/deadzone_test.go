package deadzone

import "testing"

func TestSaturationDetectionAndRange(t *testing.T) {
	wave := []float64{0.1, 0.9, 0.995, 1.0, 0.999, 0.2}
	if !DetectSaturation(wave, 1.0, 0.98, 3) {
		t.Fatal("expected a flat near-full-scale run to be saturated")
	}
	start, end, ok := SaturatedRange(wave, 1.0, 0.98)
	if !ok || start != 2 || end != 4 {
		t.Fatalf("saturated range = (%d, %d, %t), want (2, 4, true)", start, end, ok)
	}
}

func TestMergerCombinesOverlappingAndNearbyZones(t *testing.T) {
	merger := NewMerger()
	merger.Add(Zone{StartSample: 20, EndSample: 30, Reason: ReasonSaturated})
	merger.Add(Zone{StartSample: 35, EndSample: 40, Reason: ReasonBaselineDrift})
	merger.Add(Zone{StartSample: 100, EndSample: 105, Reason: ReasonUnresolvablePileup})

	zones := merger.Zones()
	if len(zones) != 2 {
		t.Fatalf("merged zone count = %d, want 2", len(zones))
	}
	if zones[0].StartSample != 20 || zones[0].EndSample != 40 || zones[0].Reason != ReasonSaturated {
		t.Fatalf("first merged zone = %+v, want [20,40] with original reason", zones[0])
	}
	if merger.TotalSamples() != 27 {
		t.Fatalf("total covered samples = %d, want 27", merger.TotalSamples())
	}
}
