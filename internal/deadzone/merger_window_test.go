package deadzone

import "testing"

// TestMergerKeepsZonesFromDifferentWindowsApart 验证合并器不会把不同窗口的
// 本地样本坐标混作同一坐标系：两个窗口在各自的本地样本位置（恰好都是 10~20）
// 产生死区时，应各自保留为独立的死区，而不是被压成一条。
func TestMergerKeepsZonesFromDifferentWindowsApart(t *testing.T) {
	merger := NewMerger()
	// 窗口 A：本地饱和区 [10,20]，绝对起点 1000ns。
	merger.Add(Zone{StartSample: 10, EndSample: 20, Reason: ReasonSaturated, WindowID: "win-a", WindowStartNs: 1000})
	// 窗口 B：本地饱和区 [10,20]（与 A 的本地位置重合），绝对起点 5000ns。
	merger.Add(Zone{StartSample: 10, EndSample: 20, Reason: ReasonSaturated, WindowID: "win-b", WindowStartNs: 5000})

	zones := merger.Zones()
	if len(zones) != 2 {
		t.Fatalf("merged zone count = %d, want 2 (one per window)", len(zones))
	}
	if zones[0].WindowID != "win-a" || zones[0].StartSample != 10 || zones[0].EndSample != 20 {
		t.Fatalf("zone[0] = %+v, want win-a [10,20]", zones[0])
	}
	if zones[1].WindowID != "win-b" || zones[1].StartSample != 10 || zones[1].EndSample != 20 {
		t.Fatalf("zone[1] = %+v, want win-b [10,20]", zones[1])
	}
}

// TestMergerStillMergesWithinSameWindow 验证同一窗口内相邻/重叠的死区仍被合并。
func TestMergerStillMergesWithinSameWindow(t *testing.T) {
	merger := NewMerger()
	merger.Add(Zone{StartSample: 20, EndSample: 30, Reason: ReasonSaturated, WindowID: "win-a", WindowStartNs: 1000})
	merger.Add(Zone{StartSample: 35, EndSample: 40, Reason: ReasonBaselineDrift, WindowID: "win-a", WindowStartNs: 1000})

	zones := merger.Zones()
	if len(zones) != 1 {
		t.Fatalf("merged zone count = %d, want 1 (same-window merge)", len(zones))
	}
	if zones[0].StartSample != 20 || zones[0].EndSample != 40 {
		t.Fatalf("merged zone = %+v, want [20,40]", zones[0])
	}
	if zones[0].Reason != ReasonSaturated {
		t.Fatalf("merged reason = %q, want %q", zones[0].Reason, ReasonSaturated)
	}
}
