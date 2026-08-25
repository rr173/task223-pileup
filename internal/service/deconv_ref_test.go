package service

import (
	"math"
	"testing"

	"task223-pileup/internal/store"
)

// addAt 把幅度为 amp、中心 c、宽度 sigma 的高斯叠加到 samples 上。
func addAt(samples []float64, c, amp, sigma float64) {
	for i := range samples {
		x := float64(i) - c
		samples[i] += amp * math.Exp(-(x*x)/(2*sigma*sigma))
	}
}

// 验证：自动锁定参考脉冲时跳过含堆积的窗口，优先从可确认的孤立脉冲建立参考。
//
// 第一段窗口含两个靠近的脉冲（堆积），后面才出现真正孤立的脉冲。
// 修复前服务会从第一段堆积波形的最强峰提取参考形状，导致形状被相邻脉冲污染；
// 修复后应跳过堆积窗口，从孤立脉冲窗口提取参考。
func TestAutoLockSkipsPiledWindow(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatal(err)
	}

	// 工况：500 MHz 采样率，死区 40ns -> 20 样本。脉冲 sigma 4 -> 半高宽约 9 样本。
	run, err := app.Runs.Create("参考自动锁定测试", "先堆积后孤立", "NaI", 5e8, 40)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	const (
		n        = 200
		sigma    = 4.0
		windowNs = 400
	)
	baseNoise := func(s []float64) {
		for i := range s {
			s[i] = 0.02
		}
	}

	// 窗口 1：两个靠近的脉冲（间距 12 < 死区 20，但大于半高宽 ~9，形成可分辨的堆积）。
	piled := make([]float64, n)
	baseNoise(piled)
	addAt(piled, 90, 0.9, sigma)
	addAt(piled, 102, 0.7, sigma) // 堆积，最强峰附近被相邻脉冲污染
	if _, err := app.Windows.Ingest(run, 1, 0, windowNs, piled); err != nil {
		t.Fatalf("ingest piled: %v", err)
	}

	// 窗口 2：真正孤立的脉冲，中心 100，幅度 0.8（应作为参考源）。
	iso := make([]float64, n)
	baseNoise(iso)
	addAt(iso, 100, 0.8, sigma)
	if _, err := app.Windows.Ingest(run, 2, windowNs, windowNs, iso); err != nil {
		t.Fatalf("ingest isolated: %v", err)
	}

	run, err = app.Runs.FinishReceiving(run.ID)
	if err != nil {
		t.Fatalf("finish receiving: %v", err)
	}

	// 直接走自动锁定路径（绕过 Deconvolve 的完整链，聚焦参考选择逻辑）。
	// noiseFloor 取一个小正数，驱动峰值阈值。
	ref, err := app.Deconv.lockOrExtractReference(run, 0.01)
	if err != nil {
		t.Fatalf("lockOrExtractReference: %v", err)
	}

	// 关键断言：参考脉冲应来自孤立窗口 2，而非堆积窗口 1。
	if ref.SourceWindow == "" {
		t.Fatal("reference has no source window")
	}
	srcWin, err := app.Windows.GetWindow(ref.SourceWindow)
	if err != nil {
		t.Fatalf("get source window: %v", err)
	}
	if srcWin.TriggerIndex != 2 {
		t.Errorf("auto-lock should skip the piled window 1 and pick the isolated window 2; got source trigger_index=%d",
			srcWin.TriggerIndex)
	}

	// 参考形状应以孤立脉冲为模板：峰值附近形状应接近纯净高斯归一化形状，
	// 而堆积波形提取的形状在相邻脉冲侧会明显偏高。校验参考形状与纯高斯
	// 归一化形状的吻合度，确保未被堆积污染。
	shape, err := decodeSamples(ref.Shape)
	if err != nil || len(shape) == 0 {
		t.Fatalf("decode reference shape: %v", err)
	}
	// 定位参考形状峰值在 shape 中的下标，构造以该下标为中心的纯净高斯对照。
	peakIdx := 0
	for i, v := range shape {
		if v > shape[peakIdx] {
			peakIdx = i
		}
	}
	// 参考形状峰值应已归一化到 1.0（保持既有归一化行为）。
	if math.Abs(shape[peakIdx]-1.0) > 1e-9 {
		t.Errorf("reference shape should be normalized to peak 1.0, got %v", shape[peakIdx])
	}
	pure := make([]float64, len(shape))
	for i := range pure {
		x := float64(i - peakIdx)
		pure[i] = math.Exp(-(x * x) / (2 * sigma * sigma))
	}
	// 对照形状按峰值归一化（峰值即 1.0）。
	var maxErr float64
	for i := range shape {
		d := math.Abs(shape[i] - pure[i])
		if d > maxErr {
			maxErr = d
		}
	}
	// 堆积波形提取的参考在相邻脉冲侧会出现明显偏差（>0.1）；孤立脉冲模板偏差应远小。
	if maxErr > 0.05 {
		t.Errorf("reference shape deviates from isolated gaussian template (maxErr=%v); likely extracted from piled window", maxErr)
	}

	// 宽度计算行为应保持：半高宽换算到纳秒，应与 sigma=4 的高斯一致（约 9.4 样本 -> ~19ns）。
	if ref.WidthNs <= 0 {
		t.Errorf("reference width should be positive, got %d", ref.WidthNs)
	}
}
