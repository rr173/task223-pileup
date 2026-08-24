package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"task223-pileup/internal/deadzone"
	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

// WindowService 波形窗口接收与饱和标记服务。
type WindowService struct {
	store    *store.WindowStore
	runStore *store.RunStore
}

// NewWindowService 构造窗口服务。
func NewWindowService(s *store.WindowStore, r *store.RunStore) *WindowService {
	return &WindowService{store: s, runStore: r}
}

// IngestResult 是一次窗口接收的结果。
type IngestResult struct {
	Inserted  int // 实际插入数
	Duplicate int // 重复（触发序号冲突）数
	Window    *model.WaveformWindow
}

// Ingest 接收一段波形窗口。
//
// 校验规则：
//   - 运行必须处于接收中（receiving），封存/处理中拒绝接收；
//   - 触发序号必须严格大于该运行已有最大触发序号（单调递增）；
//   - 波形样本非空。
//
// 窗口按饱和检测结果分类为 saturated（饱和）或 raw（原始）。
func (s *WindowService) Ingest(run *model.Run, triggerIndex, startTimeNs, durationNs int64, samples []float64) (*IngestResult, error) {
	if run.Status != model.RunReceiving {
		if run.Status == model.RunSealed {
			return nil, model.ErrSealed
		}
		return nil, fmt.Errorf("%w: run is %s, not receiving", model.ErrInvalidState, run.Status)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("%w: empty samples", model.ErrInvalidInput)
	}
	latest, err := s.store.LatestTrigger(run.ID)
	if err != nil {
		return nil, err
	}
	if triggerIndex <= latest {
		return &IngestResult{Inserted: 0, Duplicate: 1}, nil
	}

	// 波形摘要：基线水平（中位数）、峰值幅度。
	baselineLevel := medianOf(samples)
	peak := maxOf(samples)

	saturated := deadzone.DetectSaturation(samples, 1.0, 0.98, 4)
	status := model.WindowRaw
	if saturated {
		status = model.WindowSaturated
	}

	fp := windowFingerprint(run.ID, triggerIndex)
	now := time.Now().UTC()
	w := &model.WaveformWindow{
		ID:            "win-" + shortHash(fp),
		RunID:         run.ID,
		TriggerIndex:  triggerIndex,
		StartTimeNs:   startTimeNs,
		DurationNs:    durationNs,
		Samples:       encodeSamples(samples),
		BaselineLevel: baselineLevel,
		PeakAmplitude: peak,
		Saturated:     saturated,
		Status:        status,
		Fingerprint:   fp,
		CreatedAt:     now,
	}
	if err := s.store.Create(w); err != nil {
		if err == model.ErrDuplicate {
			return &IngestResult{Inserted: 0, Duplicate: 1}, nil
		}
		return nil, err
	}
	return &IngestResult{Inserted: 1, Duplicate: 0, Window: w}, nil
}

// ListWindows 返回某运行的窗口（按触发序号升序）。
func (s *WindowService) ListWindows(runID string) ([]model.WaveformWindow, error) {
	return s.store.ListByRun(runID)
}

// GetWindow 读取窗口。
func (s *WindowService) GetWindow(id string) (*model.WaveformWindow, error) {
	return s.store.Get(id)
}

// MarkSaturated 手动把窗口标记为饱和（排除出可恢复区）。
func (s *WindowService) MarkSaturated(windowID string) (*model.WaveformWindow, error) {
	w, err := s.store.Get(windowID)
	if err != nil {
		return nil, err
	}
	if err := s.store.UpdateClassify(w.ID, w.BaselineLevel, w.PeakAmplitude, true, model.WindowSaturated); err != nil {
		return nil, err
	}
	return s.store.Get(windowID)
}

// CountSaturated 统计某运行的饱和窗口数。
func (s *WindowService) CountSaturated(runID string) (int, error) {
	return s.store.CountSaturated(runID)
}

// Samples 解码窗口波形样本为浮点切片。
func (s *WindowService) Samples(w *model.WaveformWindow) ([]float64, error) {
	return decodeSamples(w.Samples)
}

// --- 波形样本编解码 ---

func encodeSamples(samples []float64) string {
	b, _ := json.Marshal(samples)
	return string(b)
}

func decodeSamples(raw string) ([]float64, error) {
	var out []float64
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode samples: %w", err)
	}
	return out, nil
}

// --- 摘要辅助 ---

func medianOf(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := make([]float64, len(v))
	copy(c, v)
	// 插入排序后取中位（样本量小，够用）。
	for i := 1; i < len(c); i++ {
		for j := i; j > 0 && c[j] < c[j-1]; j-- {
			c[j], c[j-1] = c[j-1], c[j]
		}
	}
	mid := len(c) / 2
	if len(c)%2 == 1 {
		return c[mid]
	}
	return (c[mid-1] + c[mid]) / 2
}

func maxOf(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func windowFingerprint(runID string, triggerIndex int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", runID, triggerIndex)))
	return hex.EncodeToString(h[:])
}
