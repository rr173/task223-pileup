package detector

// PileUp 是一个脉冲堆积组：由多个靠得过近的峰构成，需解卷积分离。
type PileUp struct {
	Peaks []Peak // 组内峰值（按位置升序）
}

// PileUpDetector 堆积识别器：按死区时间把间距过近的峰归并成堆积组。
type PileUpDetector struct {
	DeadTimeSamples int // 死区时间对应的样本数（间距小于此值视为堆积）
}

// NewPileUpDetector 构造堆积识别器。
func NewPileUpDetector(deadTimeSamples int) *PileUpDetector {
	if deadTimeSamples < 1 {
		deadTimeSamples = 1
	}
	return &PileUpDetector{DeadTimeSamples: deadTimeSamples}
}

// Group 把峰值按位置分组：相邻峰样本间距 < DeadTimeSamples 的归入同一堆积组，
// 其余峰各自为孤立组。返回的分组按起始位置升序。
func (p *PileUpDetector) Group(peaks []Peak) []PileUp {
	if len(peaks) == 0 {
		return nil
	}
	var groups []PileUp
	cur := PileUp{Peaks: []Peak{peaks[0]}}
	for i := 1; i < len(peaks); i++ {
		if peaks[i].Position-cur.Peaks[len(cur.Peaks)-1].Position < p.DeadTimeSamples {
			cur.Peaks = append(cur.Peaks, peaks[i])
		} else {
			groups = append(groups, cur)
			cur = PileUp{Peaks: []Peak{peaks[i]}}
		}
	}
	groups = append(groups, cur)
	return groups
}

// IsPiled 判断一个堆积组是否为真实堆积（含多于一个峰）。
func (g PileUp) IsPiled() bool { return len(g.Peaks) > 1 }

// IsolatedPeak returns the sole peak when this group is safe to use as a
// reference pulse source.
func (g PileUp) IsolatedPeak() (Peak, bool) {
	if g.IsPiled() || len(g.Peaks) == 0 {
		return Peak{}, false
	}
	return g.Peaks[0], true
}
