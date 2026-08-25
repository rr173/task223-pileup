package deadzone

// DetectSaturation 检测一段波形是否饱和。
//
// 判据：去基线幅度达到满量程（fullScale）的 FlatRatio 以上，且出现平顶
// （连续 flatRun 个样本幅度几乎不变）。饱和窗口无法伪造分解结果，应整体
// 标记为不可恢复区。
//
// 注意：本函数直接在原始样本上判据，因此当窗口直流基线整体偏高时会误判
// ——基线本身把样本抬过阈值，而非脉冲真正打满量程。接收端分类窗口时应优先
// 使用 DetectSaturationAboveBaseline，先扣除窗口直流基线再判断。
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

// DetectSaturationAboveBaseline 检测波形在扣除窗口直流基线后是否饱和。
//
// 与 DetectSaturation 的区别在于先减去窗口直流基线（baseline，通常取窗口
// 噪声底/最小值），再按去基线幅度判断是否打满量程并出现平顶。这样当基线
// 整体偏高、但脉冲幅度并未达到量程时不会误判为饱和；而真正打满量程形成
// 平顶的信号（去基线后幅度仍接近满量程）仍被判为饱和。
func DetectSaturationAboveBaseline(wave []float64, baseline, fullScale, flatRatio float64, flatRun int) bool {
	corrected := correctedWave(wave, baseline)
	if len(corrected) == 0 || fullScale <= 0 {
		return false
	}
	threshold := fullScale * flatRatio
	run := 0
	for i, v := range corrected {
		if v >= threshold {
			if i > 0 && nearlyEqual(v, corrected[i-1], fullScale*0.01) {
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

// correctedWave 返回扣除窗口直流基线（baseline）后的波形，负值截断为 0。
// 复用解卷积链路使用的 subtractBaseline 语义，避免分类与解卷积对“去基线”
// 的口径不一致。
func correctedWave(wave []float64, baseline float64) []float64 {
	if len(wave) == 0 {
		return nil
	}
	out := make([]float64, len(wave))
	for i, v := range wave {
		v -= baseline
		if v < 0 {
			v = 0
		}
		out[i] = v
	}
	return out
}

// SaturatedRangeAboveBaseline locates the first full-scale run after removing
// the window's DC baseline.
func SaturatedRangeAboveBaseline(wave []float64, baseline, fullScale, flatRatio float64) (start, end int, ok bool) {
	corrected := correctedWave(wave, baseline)
	return SaturatedRange(corrected, fullScale, flatRatio)
}

func nearlyEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
