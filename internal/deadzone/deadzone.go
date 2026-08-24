// Package deadzone 提供不可恢复时间区（死区）的标记与合并。
// 死区来源有三类：信号饱和、基线漂移过大、脉冲堆积无法分离。合并器
// 把相邻或重叠的死区归并为连续区间，供计数阶段从有效观察时间中扣除。
package deadzone

import "sort"

// 死区原因常量。
const (
	ReasonSaturated          = "saturated"           // 信号饱和（超出量程）
	ReasonBaselineDrift      = "baseline_drift"      // 基线漂移过大
	ReasonUnresolvablePileup = "unresolvable_pileup" // 堆积不可分离
)

// Zone 是一个不可恢复区（样本坐标，相对窗口起点）。
type Zone struct {
	StartSample int    // 起始样本
	EndSample   int    // 结束样本（含）
	Reason      string // 原因
	OriginNs    int64  // 所属波形窗口的绝对起点，避免跨窗口坐标合并
}

// SaturatedZone returns the precise saturated interval in a waveform.
func SaturatedZone(wave []float64, baseline, fullScale, flatRatio float64) (Zone, bool) {
	start, end, ok := SaturatedRangeAboveBaseline(wave, baseline, fullScale, flatRatio)
	if !ok {
		return Zone{}, false
	}
	return Zone{StartSample: start, EndSample: end, Reason: ReasonSaturated}, true
}

// Merger 死区标记与合并器。
type Merger struct {
	MergeGap int // 相邻死区间隔小于此样本数则合并
	zones    []Zone
}

// NewMerger 构造死区合并器（默认合并间隔 8 样本）。
func NewMerger() *Merger { return &Merger{MergeGap: 8} }

// Add 添加一个死区并自动合并重叠/相邻区间（按起始位置有序）。
func (m *Merger) Add(z Zone) {
	if z.EndSample < z.StartSample {
		z.EndSample = z.StartSample
	}
	m.zones = append(m.zones, z)
}

// Zones 返回合并后的死区列表（按起始样本升序）。
func (m *Merger) Zones() []Zone {
	if len(m.zones) == 0 {
		return nil
	}
	sorted := make([]Zone, len(m.zones))
	copy(sorted, m.zones)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartSample < sorted[j].StartSample })

	var merged []Zone
	cur := sorted[0]
	for _, z := range sorted[1:] {
		if z.OriginNs == cur.OriginNs && z.StartSample-cur.EndSample <= m.MergeGap {
			if z.EndSample > cur.EndSample {
				cur.EndSample = z.EndSample
			}
			if cur.Reason == "" && z.Reason != "" {
				cur.Reason = z.Reason
			}
		} else {
			merged = append(merged, cur)
			cur = z
		}
	}
	merged = append(merged, cur)
	return merged
}

// TotalSamples 返回合并后死区覆盖的总样本数。
func (m *Merger) TotalSamples() int {
	total := 0
	for _, z := range m.Zones() {
		total += z.EndSample - z.StartSample + 1
	}
	return total
}
