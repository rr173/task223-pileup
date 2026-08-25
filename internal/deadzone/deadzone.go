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
//
// 每个波形窗口有独立的本地样本坐标系（样本索引从窗口起点计起）。Zone 携带
// 所属窗口 ID 与窗口绝对起点，使合并器只在同一窗口内合并；保存时再把本地
// 样本偏移叠加到窗口绝对起点上，从而保留每个窗口的绝对时间来源。
type Zone struct {
	StartSample   int    // 起始样本（相对窗口起点）
	EndSample     int    // 结束样本（含）
	Reason        string // 原因
	WindowID      string // 所属窗口 ID：同窗口内的死区才合并；空表示无窗口来源（按单一坐标系合并）
	WindowStartNs int64  // 窗口绝对起点（纳秒）：保存时叠加到样本偏移上还原绝对时间
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

// Zones 返回合并后的死区列表（按窗口分组，窗口内按起始样本升序）。
//
// 合并只在同一窗口坐标系内发生：不同窗口的本地样本坐标互不可比，
// 简单按样本索引排序会把不同窗口中位于同一本地位置的死区误判为重叠，
// 从而丢掉本应分别持久化的死区。因此先按 WindowID 分桶，再在各桶内
// 按（StartSample, EndSample）排序并合并相邻/重叠区间。
// 空 WindowID（无窗口来源）自成一组，按单一坐标系合并。
func (m *Merger) Zones() []Zone {
	if len(m.zones) == 0 {
		return nil
	}
	// 按 WindowID 稳定分组，保持窗口的出现顺序。
	order := make([]string, 0, len(m.zones))
	buckets := make(map[string][]Zone)
	for _, z := range m.zones {
		if _, ok := buckets[z.WindowID]; !ok {
			order = append(order, z.WindowID)
		}
		buckets[z.WindowID] = append(buckets[z.WindowID], z)
	}

	var merged []Zone
	for _, wid := range order {
		group := buckets[wid]
		sort.Slice(group, func(i, j int) bool {
			if group[i].StartSample != group[j].StartSample {
				return group[i].StartSample < group[j].StartSample
			}
			return group[i].EndSample < group[j].EndSample
		})
		cur := group[0]
		for _, z := range group[1:] {
			if z.StartSample-cur.EndSample <= m.MergeGap {
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
	}
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
