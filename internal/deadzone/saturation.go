package deadzone

// DetectSaturation 检测一段波形是否饱和。
//
// 判据：去基线幅度达到满量程（fullScale）的 FlatRatio 以上，且出现平顶
// （连续 flatRun 个样本幅度几乎不变）。饱和窗口无法伪造分解结果，应整体
// 标记为不可恢复区。
func DetectSaturation(wave []float64, fullScale, flatRatio float64, flatRun int) bool {
	if len(wave) == 0 || fullScale <= 0 {
		return false
	}
	threshold := fullScale * flatRatio
	run := 0
	for i, v := range wave {
		if v >= threshold {
			if i > 0 && nearlyEqual(v, wave[i-1], fullScale*0.01) {
				run++
			} else {
				run = 1
			}
			if run >= flatRun {
				return true
			}
		} else {
			run = 0
		}
	}
	return false
}

// DetectSaturationAboveBaseline 判定去除直流基线后的波形是否饱和。
// 探测器的满量程是相对基线的幅度；直接把带基线的原始电平与满量程比较，
// 会把高基线噪声误报成饱和。
func DetectSaturationAboveBaseline(wave []float64, baseline, fullScale, flatRatio float64, flatRun int) bool {
	if len(wave) == 0 {
		return false
	}
	corrected := make([]float64, len(wave))
	for i, sample := range wave {
		corrected[i] = sample - baseline
		if corrected[i] < 0 {
			corrected[i] = 0
		}
	}
	return DetectSaturation(corrected, fullScale, flatRatio, flatRun)
}

// SaturatedRange 定位波形中达到满量程的连续样本区间（用于标记死区）。
// 返回第一个满足条件的 [start, end] 样本区间；无饱和返回 ok=false。
func SaturatedRange(wave []float64, fullScale, flatRatio float64) (start, end int, ok bool) {
	if len(wave) == 0 || fullScale <= 0 {
		return 0, 0, false
	}
	threshold := fullScale * flatRatio
	i := 0
	for i < len(wave) {
		if wave[i] >= threshold {
			j := i
			for j+1 < len(wave) && wave[j+1] >= threshold {
				j++
			}
			return i, j, true
		}
		i++
	}
	return 0, 0, false
}

// SaturatedRangeAboveBaseline locates the first full-scale run after removing
// the window's DC baseline.
func SaturatedRangeAboveBaseline(wave []float64, baseline, fullScale, flatRatio float64) (start, end int, ok bool) {
	if len(wave) == 0 {
		return 0, 0, false
	}
	corrected := make([]float64, len(wave))
	for i, sample := range wave {
		corrected[i] = sample - baseline
		if corrected[i] < 0 {
			corrected[i] = 0
		}
	}
	return SaturatedRange(corrected, fullScale, flatRatio)
}

func nearlyEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
