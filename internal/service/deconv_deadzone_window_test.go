package service

import (
	"math"
	"math/rand"
	"testing"

	"task223-pileup/internal/store"
)

const (
	dzSampleRate = 5e8 // 500 MHz
	dzWindowNs   = 400
	dzDeadTimeNs = 40
	dzSamples    = 200
)

func dzAddPulse(samples []float64, center, amp, sigma float64) {
	for i := range samples {
		x := float64(i) - center
		samples[i] += amp * math.Exp(-(x*x)/(2*sigma*sigma))
	}
}

// dzIsolated 构造含单个孤立脉冲的窗口（作参考脉冲来源）。
func dzIsolated(rng *rand.Rand, center float64) []float64 {
	s := make([]float64, dzSamples)
	for i := range s {
		s[i] = 0.02 + rng.NormFloat64()*0.01
	}
	dzAddPulse(s, center, 0.8, 4.0)
	return s
}

// dzFlat 构造一个平坦波形（用于手动标记为饱和的窗口）。
func dzFlat(rng *rand.Rand) []float64 {
	s := make([]float64, dzSamples)
	for i := range s {
		s[i] = 0.02 + rng.NormFloat64()*0.01
	}
	return s
}

// TestDeconvolvePersistsDeadZonePerWindowWithAbsoluteTime 验证：两个不同波形窗口
// 在各自本地样本位置产生死区时，服务不会把它们当成同一坐标系中的相邻区间合并，
// 而是各自持久化一条死区，且保存时保留每个窗口的绝对时间起点。
//
// 回归场景：旧实现用一个共享 Merger，按裸样本索引排序，两个窗口中位于同一
// 本地位置的死区被误判为重叠而合并成一条；且 StartTimeNs 只算了样本偏移、
// 丢了窗口绝对起点。本测试确保两条死区都落库且绝对时间正确。
func TestDeconvolvePersistsDeadZonePerWindowWithAbsoluteTime(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewSource(7))

	run, err := app.Runs.Create("dz-per-window", "two saturated windows", "NaI", dzSampleRate, dzDeadTimeNs)
	if err != nil {
		t.Fatal(err)
	}

	// 窗口 1：含孤立脉冲，作参考脉冲来源（触发序号从 1 起：序号 0 会被
	// LatestTrigger 初值 0 当作重复而丢弃，故不用 0）。
	if _, err := app.Windows.Ingest(run, 1, 0, dzWindowNs, dzIsolated(rng, 100)); err != nil {
		t.Fatal(err)
	}
	// 窗口 2：手动标记为饱和（绝对起点 1000ns）。
	if _, err := app.Windows.Ingest(run, 2, 1000, dzWindowNs, dzFlat(rng)); err != nil {
		t.Fatal(err)
	}
	// 窗口 3：手动标记为饱和（绝对起点 2000ns），与窗口 2 的本地饱和区位置相同。
	if _, err := app.Windows.Ingest(run, 3, 2000, dzWindowNs, dzFlat(rng)); err != nil {
		t.Fatal(err)
	}

	// 标记窗口 2、3 为饱和（走饱和分支，各自产生整窗死区）。
	wins, err := app.Windows.ListWindows(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range wins {
		if w.TriggerIndex == 2 || w.TriggerIndex == 3 {
			if _, err := app.Windows.MarkSaturated(w.ID); err != nil {
				t.Fatalf("mark window %d saturated: %v", w.TriggerIndex, err)
			}
		}
	}

	// 结束接收 -> 处理中 -> 解卷积。
	if _, err := app.Runs.FinishReceiving(run.ID); err != nil {
		t.Fatal(err)
	}
	res, err := app.Deconv.Deconvolve(run.ID)
	if err != nil {
		t.Fatalf("deconvolve: %v", err)
	}

	// 两个饱和窗口必须各自落库一条死区（不能被压成一条）。
	if res.DeadZones != 2 {
		t.Fatalf("dead zones = %d, want 2 (one per saturated window)", res.DeadZones)
	}
	zones, err := app.Deconv.ListDeadZones(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 2 {
		t.Fatalf("persisted dead zones = %d, want 2", len(zones))
	}

	periodNs := int64(1e9 / dzSampleRate) // 每样本 2ns

	// 验证绝对时间起点：两条死区的绝对起点应分别是窗口 1（1000ns）与窗口 2（2000ns），
	// 而非都被压回 0（旧实现丢失窗口绝对起点）。
	gotStarts := map[int64]bool{}
	for _, z := range zones {
		gotStarts[z.StartTimeNs] = true
		if z.EndTimeNs < z.StartTimeNs {
			t.Errorf("dead zone [%d,%d] inverted", z.StartTimeNs, z.EndTimeNs)
		}
	}
	if !gotStarts[1000] || !gotStarts[2000] {
		t.Fatalf("dead zone absolute starts = %v, want both 1000 and 2000 (per-window origin preserved)", gotStarts)
	}
	_ = periodNs
}
