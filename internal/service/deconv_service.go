package service

import (
	"fmt"
	"time"

	"task223-pileup/internal/baseline"
	"task223-pileup/internal/deadzone"
	"task223-pileup/internal/deconv"
	"task223-pileup/internal/detector"
	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

// DeconvService 脉冲堆积解卷积编排服务：把基线估计、峰值检测、堆积识别、
// 受约束解卷积与死区标记串成完整处理链。
type DeconvService struct {
	runStore      *store.RunStore
	windowStore   *store.WindowStore
	pulseStore    *store.PulseStore
	deadZoneStore *store.DeadZoneStore
	baselineStore *store.BaselineStore

	estimator     *baseline.Estimator
	driftDetector *baseline.DriftDetector
	deconvolver   *deconv.Deconvolver
	constraints   deconv.Constraints
}

// NewDeconvService 构造解卷积服务。
func NewDeconvService(runStore *store.RunStore, windowStore *store.WindowStore,
	pulseStore *store.PulseStore, deadZoneStore *store.DeadZoneStore, baselineStore *store.BaselineStore) *DeconvService {
	return &DeconvService{
		runStore:      runStore,
		windowStore:   windowStore,
		pulseStore:    pulseStore,
		deadZoneStore: deadZoneStore,
		baselineStore: baselineStore,
		estimator:     baseline.NewEstimator(),
		driftDetector: baseline.NewDriftDetector(),
		deconvolver:   deconv.NewDeconvolver(),
		constraints:   deconv.DefaultConstraints(),
	}
}

// DeconvResult 是一次解卷积处理的结果摘要。
type DeconvResult struct {
	WindowCount          int // 处理窗口数
	PiledWindowCount     int // 堆积窗口数
	SaturatedWindowCount int // 饱和窗口数
	RecoveredPulses      int // 恢复脉冲数
	InseparablePulses    int // 不可分离脉冲数
	DeadZones            int // 死区数
}

// EstimateBaseline 估计运行基线并落库。
func (s *DeconvService) EstimateBaseline(runID string) (*model.BaselineRecord, error) {
	run, err := s.runStore.Get(runID)
	if err != nil {
		return nil, err
	}
	windows, err := s.windowStore.ListByRun(runID)
	if err != nil {
		return nil, err
	}
	// 只用非饱和窗口估计基线。
	var waves [][]float64
	for _, w := range windows {
		if w.Status == model.WindowSaturated {
			continue
		}
		samples, err := decodeSamples(w.Samples)
		if err != nil {
			return nil, err
		}
		waves = append(waves, samples)
	}
	res := s.estimator.Estimate(waves)
	rec := &model.BaselineRecord{
		ID:          "base-" + run.ID,
		RunID:       runID,
		Level:       res.Level,
		DriftSlope:  res.DriftSlope,
		NoiseFloor:  res.NoiseFloor,
		WindowCount: res.WindowCount,
		Locked:      false,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.baselineStore.UpsertBaseline(rec); err != nil {
		return nil, err
	}
	_ = run
	return rec, nil
}

// GetBaseline 读取运行基线。
func (s *DeconvService) GetBaseline(runID string) (*model.BaselineRecord, error) {
	return s.baselineStore.GetBaseline(runID)
}

// LockReference 从指定窗口的孤立脉冲锁定参考脉冲形状。
func (s *DeconvService) LockReference(runID, windowID string) (*model.ReferencePulse, error) {
	run, err := s.runStore.Get(runID)
	if err != nil {
		return nil, err
	}
	w, err := s.windowStore.Get(windowID)
	if err != nil {
		return nil, err
	}
	if w.RunID != runID {
		return nil, fmt.Errorf("%w: window does not belong to run", model.ErrInvalidInput)
	}
	samples, err := decodeSamples(w.Samples)
	if err != nil {
		return nil, err
	}
	shape, peakPos := s.extractShape(samples, w.BaselineLevel, s.deadTimeSamples(run))
	if shape == nil {
		return nil, fmt.Errorf("%w: no usable pulse in window", model.ErrDeconvFailed)
	}
	widthSamples := deconv.HalfWidthSamples(shape)
	now := time.Now().UTC()
	ref := &model.ReferencePulse{
		ID:           "ref-" + run.ID,
		RunID:        runID,
		Amplitude:    1.0,
		WidthNs:      deconv.WidthNsAtSampleRate(widthSamples, run.SampleRateHz),
		Shape:        encodeSamples(shape),
		SourceWindow: windowID,
		LockedAt:     now,
		CreatedAt:    now,
	}
	if err := s.baselineStore.UpsertReferencePulse(ref); err != nil {
		return nil, err
	}
	_ = peakPos
	return ref, nil
}

// Deconvolve 执行完整解卷积链：
//
//  1. 校验运行处于 processing；
//  2. 估计基线；
//  3. 锁定（或自动提取）参考脉冲；
//  4. 清空旧脉冲/死区；
//  5. 逐窗口：去基线 → 峰值检测 → 堆积识别 → 解卷积/死区标记；
//  6. 状态流转 processing → pending_review。
func (s *DeconvService) Deconvolve(runID string) (*DeconvResult, error) {
	run, err := s.runStore.Get(runID)
	if err != nil {
		return nil, err
	}
	if run.Status != model.RunProcessing {
		return nil, fmt.Errorf("%w: run is %s, not processing", model.ErrInvalidState, run.Status)
	}

	// 步骤 2：估计基线。
	bl, err := s.EstimateBaseline(runID)
	if err != nil {
		return nil, err
	}

	// 步骤 3：参考脉冲（优先取已锁定，否则自动提取）。
	ref, err := s.lockOrExtractReference(run)
	if err != nil {
		return nil, err
	}
	refShape, err := decodeSamples(ref.Shape)
	if err != nil || len(refShape) == 0 {
		return nil, fmt.Errorf("%w: invalid reference pulse", model.ErrDeconvFailed)
	}

	// 步骤 4：清空旧结果。
	if err := s.pulseStore.DeleteByRun(runID); err != nil {
		return nil, err
	}
	if err := s.deadZoneStore.DeleteByRun(runID); err != nil {
		return nil, err
	}

	// 步骤 5：逐窗口处理。
	windows, err := s.windowStore.ListByRun(runID)
	if err != nil {
		return nil, err
	}
	res := &DeconvResult{WindowCount: len(windows)}
	merger := deadzone.NewMerger()
	dtSamples := s.deadTimeSamples(run)
	threshold := bl.NoiseFloor * 3
	if threshold < 0.05 {
		threshold = 0.05
	}
	peakDet := detector.NewPeakDetector(threshold, maxInt(2, dtSamples/2))
	pileUpDet := detector.NewPileUpDetector(dtSamples)
	groupIdx := 0

	for _, w := range windows {
		samples, err := decodeSamples(w.Samples)
		if err != nil {
			return nil, err
		}
		switch w.Status {
		case model.WindowSaturated:
			res.SaturatedWindowCount++
			// 死区范围同样在扣除窗口直流基线后定位，与接收端分类口径一致：
			// 真正打满量程的平顶段去基线后仍接近满量程，定位到该段；若窗口
			// 整体已被标记饱和但去基线后未达阈值（极端裁剪），则整窗计为死区。
			zone, ok := deadzone.SaturatedZone(samples, dcBaselineOf(samples), 1.0, 0.98)
			if !ok {
				zone = deadzone.Zone{StartSample: 0, EndSample: len(samples) - 1, Reason: deadzone.ReasonSaturated}
			}
			s.addZone(merger, run, w, zone.StartSample, zone.EndSample, zone.Reason)
			continue
		}

		// 去基线。
		wave := subtractBaseline(samples, w.BaselineLevel)

		// 基线漂移判定：窗口基线相对全局基线偏离过大则整窗标记死区。
		if s.driftDetector.ClassifyWithSlope(bl.Level, baseline.DriftWindow{WindowBase: w.BaselineLevel}, bl.DriftSlope) || s.driftDetector.SlopeExceeded(bl.DriftSlope) {
			s.addZone(merger, run, w, 0, len(samples), deadzone.ReasonBaselineDrift)
			continue
		}

		peaks := peakDet.Detect(wave)
		groups := pileUpDet.Group(peaks)
		for _, g := range groups {
			groupIdx++
			if !g.IsPiled() {
				// 孤立脉冲：直接确认。
				pk := g.Peaks[0]
				s.addPulse(run, w, pk.Position, pk.Amplitude, 0, model.PulseSeparated, 0, 0.99)
				continue
			}
			// 堆积：解卷积。最小间隔取脉冲可分辨宽度（死区时间的 1/4），
			// 而非死区时间本身——堆积脉冲间距小于死区但大于可分辨宽度。
			res.PiledWindowCount++
			minSep := maxInt(2, dtSamples/4)
			s.constraints.MinSeparation = minSep
			result := s.deconvolver.Deconvolve(wave, refShape, minSep, bl.NoiseFloor)
			filtered := s.constraints.Filter(result.Pulses)
			if !s.constraints.Separable(result) || !deconv.NonNegative(filtered) {
				// 不可分离：标记死区。
				start, end := s.groupSpan(g, len(samples))
				s.addZone(merger, run, w, start, end, deadzone.ReasonUnresolvablePileup)
				res.InseparablePulses++
				// 记录一个不可分离脉冲事件，供复核。
				s.addPulse(run, w, g.Peaks[0].Position, g.Peaks[0].Amplitude, groupIdx, model.PulseInseparable, result.Residual, 0.1)
				continue
			}
			for _, rp := range filtered {
				s.addPulse(run, w, rp.Position, rp.Amplitude, groupIdx, model.PulseSeparated, result.Residual, 0.9)
				res.RecoveredPulses++
			}
		}
	}

	// 写死区。
	for _, z := range merger.Zones() {
		dz := &model.DeadZone{
			ID:          fmt.Sprintf("dz-%s-%d-%d-%d", runID, z.OriginNs, z.StartSample, z.EndSample),
			RunID:       runID,
			StartTimeNs: z.OriginNs + s.sampleToNs(run, z.StartSample),
			EndTimeNs:   z.OriginNs + s.sampleToNs(run, z.EndSample),
			Reason:      z.Reason,
			CreatedAt:   time.Now().UTC(),
		}
		if err := s.deadZoneStore.Create(dz); err != nil {
			return nil, err
		}
		res.DeadZones++
	}

	// 步骤 6：状态流转。
	if _, err := s.runStore.UpdateStatus(runID, model.RunProcessing, model.RunPendingReview); err != nil {
		return nil, err
	}
	return res, nil
}

// --- 脉冲查询与复核 ---

// ListPulses 返回某运行的脉冲。
func (s *DeconvService) ListPulses(runID string) ([]model.PulseEvent, error) {
	return s.pulseStore.ListByRun(runID)
}

// GetPulse 读取脉冲。
func (s *DeconvService) GetPulse(id string) (*model.PulseEvent, error) {
	return s.pulseStore.Get(id)
}

// ConfirmPulse 把已分离脉冲确认。
func (s *DeconvService) ConfirmPulse(id string) (*model.PulseEvent, error) {
	p, err := s.pulseStore.Get(id)
	if err != nil {
		return nil, err
	}
	if p.Status != model.PulseSeparated {
		return nil, fmt.Errorf("%w: pulse is %s, not separated", model.ErrInvalidState, p.Status)
	}
	sealed, err := s.runStore.IsSealed(p.RunID)
	if err != nil {
		return nil, err
	}
	if sealed {
		return nil, model.ErrSealed
	}
	if err := s.pulseStore.UpdateStatusIfRunMutable(id, p.RunID, model.PulseConfirmed); err != nil {
		return nil, err
	}
	return s.pulseStore.Get(id)
}

// RejectPulse 把脉冲否决为不可分离。
func (s *DeconvService) RejectPulse(id string) (*model.PulseEvent, error) {
	p, err := s.pulseStore.Get(id)
	if err != nil {
		return nil, err
	}
	if p.Status == model.PulseConfirmed {
		return nil, fmt.Errorf("%w: confirmed pulse cannot be rejected", model.ErrInvalidState)
	}
	sealed, err := s.runStore.IsSealed(p.RunID)
	if err != nil {
		return nil, err
	}
	if sealed {
		return nil, model.ErrSealed
	}
	if err := s.pulseStore.UpdateStatusIfRunMutable(id, p.RunID, model.PulseInseparable); err != nil {
		return nil, err
	}
	return s.pulseStore.Get(id)
}

// ListDeadZones 返回某运行的死区。
func (s *DeconvService) ListDeadZones(runID string) ([]model.DeadZone, error) {
	return s.deadZoneStore.ListByRun(runID)
}

// --- 内部辅助 ---

// deadTimeSamples 把死区时间换算为样本数。
func (s *DeconvService) deadTimeSamples(run *model.Run) int {
	periodNs := 1e9 / run.SampleRateHz
	n := int(float64(run.DeadTimeNs) / periodNs)
	if n < 1 {
		n = 1
	}
	return n
}

// sampleToNs 把样本索引换算为相对窗口起点的纳秒偏移。
func (s *DeconvService) sampleToNs(run *model.Run, sample int) int64 {
	periodNs := 1e9 / run.SampleRateHz
	return int64(float64(sample) * periodNs)
}

// extractShape 从波形中提取归一化参考脉冲形状（返回形状与峰位置）。
func (s *DeconvService) extractShape(samples []float64, base float64, radius int) ([]float64, int) {
	wave := subtractBaseline(samples, base)
	// 用最强峰作为参考脉冲源。
	pos, amp := 0, 0.0
	for i, v := range wave {
		if v > amp {
			amp = v
			pos = i
		}
	}
	if amp <= 0 {
		return nil, 0
	}
	if radius < 2 {
		radius = 8
	}
	return deconv.ExtractReferenceForPeak(wave, pos, radius), pos
}

// lockOrExtractReference 返回参考脉冲：优先已锁定，否则从孤立脉冲自动提取。
func (s *DeconvService) lockOrExtractReference(run *model.Run) (*model.ReferencePulse, error) {
	if ref, err := s.baselineStore.GetReferencePulse(run.ID); err == nil {
		return ref, nil
	}
	// 自动提取：从第一个含孤立脉冲的窗口提取。
	windows, err := s.windowStore.ListByRun(run.ID)
	if err != nil {
		return nil, err
	}
	dtSamples := s.deadTimeSamples(run)
	for _, w := range windows {
		if w.Status == model.WindowSaturated {
			continue
		}
		samples, err := decodeSamples(w.Samples)
		if err != nil {
			continue
		}
		wave := subtractBaseline(samples, w.BaselineLevel)
		peaks := detector.NewPeakDetector(0.05, maxInt(2, dtSamples/2)).Detect(wave)
		groups := detector.NewPileUpDetector(dtSamples).Group(peaks)
		var isolated detector.Peak
		found := false
		for _, group := range groups {
			if peak, ok := group.IsolatedPeak(); ok {
				isolated = peak
				found = true
				break
			}
		}
		if !found {
			continue
		}
		shape := deconv.ExtractReferenceForPeak(wave, isolated.Position, dtSamples)
		if shape == nil {
			continue
		}
		widthSamples := deconv.HalfWidthSamples(shape)
		now := time.Now().UTC()
		ref := &model.ReferencePulse{
			ID:           "ref-" + run.ID,
			RunID:        run.ID,
			Amplitude:    1.0,
			WidthNs:      deconv.WidthNsAtSampleRate(widthSamples, run.SampleRateHz),
			Shape:        encodeSamples(shape),
			SourceWindow: w.ID,
			LockedAt:     now,
			CreatedAt:    now,
		}
		if err := s.baselineStore.UpsertReferencePulse(ref); err != nil {
			return nil, err
		}
		return ref, nil
	}
	return nil, fmt.Errorf("%w: no reference pulse available", model.ErrDeconvFailed)
}

// addPulse 写入一个脉冲事件。
func (s *DeconvService) addPulse(run *model.Run, w model.WaveformWindow, pos int, amp float64, groupIdx int, status string, residual, confidence float64) {
	now := time.Now().UTC()
	p := &model.PulseEvent{
		ID:            "pls-" + shortHash(fmt.Sprintf("%s-%d", w.ID, pos)),
		RunID:         run.ID,
		WindowID:      w.ID,
		ArrivalTimeNs: s.sampleToNs(run, pos),
		Amplitude:     amp,
		GroupIndex:    groupIdx,
		Status:        status,
		ResidualRatio: residual,
		Confidence:    confidence,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_ = s.pulseStore.Create(p)
}

// addZone 把一个死区（样本坐标）加入合并器。
func (s *DeconvService) addZone(m *deadzone.Merger, run *model.Run, w model.WaveformWindow, start, end int, reason string) {
	m.Add(deadzone.Zone{StartSample: start, EndSample: end, Reason: reason, OriginNs: w.StartTimeNs})
}

// groupSpan 返回堆积组覆盖的样本区间。
func (s *DeconvService) groupSpan(g detector.PileUp, totalSamples int) (int, int) {
	if len(g.Peaks) == 0 {
		return 0, 0
	}
	start := g.Peaks[0].Position
	end := g.Peaks[len(g.Peaks)-1].Position
	if start > 0 {
		start--
	}
	if end < totalSamples-1 {
		end++
	}
	return start, end
}

// --- 纯函数辅助 ---

// subtractBaseline 从波形中减去基线，负值截断为 0。
func subtractBaseline(samples []float64, base float64) []float64 {
	out := make([]float64, len(samples))
	for i, v := range samples {
		out[i] = v - base
		if out[i] < 0 {
			out[i] = 0
		}
	}
	return out
}

// dcBaselineOf 返回窗口直流基线（噪声底，取最小样本），用于饱和判据中
// 扣除窗口直流偏置，与接收端分类保持同口径。
func dcBaselineOf(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	m := samples[0]
	for _, x := range samples[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
