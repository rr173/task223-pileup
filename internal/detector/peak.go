// Package detector 提供探测器波形中的峰值检测与脉冲堆积识别。
// 峰值检测用于在去基线后的波形上定位候选脉冲；堆积识别按死区时间
// 把靠得过近的峰归并为堆积组，供后续解卷积分离。
package detector

// Peak 是一个检测到的候选脉冲峰。
type Peak struct {
	Position  int     // 样本索引（相对窗口起点）
	Amplitude float64 // 去基线后的幅度
}

// PeakDetector 峰值检测器：基于一阶导数符号变化定位局部最大值。
type PeakDetector struct {
	Threshold   float64 // 幅度阈值（低于此值视为噪声，忽略）
	MinDistance int     // 相邻峰最小距离（样本），用于抑制肩峰
}

// NewPeakDetector 构造峰值检测器。
func NewPeakDetector(threshold float64, minDistance int) *PeakDetector {
	return &PeakDetector{Threshold: threshold, MinDistance: minDistance}
}

// Detect 在去基线波形上检测峰值（幅度 > 阈值且为局部最大）。
func (d *PeakDetector) Detect(wave []float64) []Peak {
	n := len(wave)
	if n < 3 {
		return nil
	}
	var peaks []Peak
	for i := 1; i < n-1; i++ {
		if wave[i] < d.Threshold {
			continue
		}
		// 局部最大：前一阶上升、后一阶下降。
		if wave[i] >= wave[i-1] && wave[i] >= wave[i+1] {
			// 抑制肩峰：与上一峰的样本距离小于 MinDistance 时，保留幅度更大者。
			if len(peaks) > 0 && i-peaks[len(peaks)-1].Position < d.MinDistance {
				last := &peaks[len(peaks)-1]
				if wave[i] > last.Amplitude {
					last.Position = i
					last.Amplitude = wave[i]
				}
				continue
			}
			peaks = append(peaks, Peak{Position: i, Amplitude: wave[i]})
		}
	}
	return peaks
}
