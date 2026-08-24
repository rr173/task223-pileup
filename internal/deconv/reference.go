package deconv

// Reference 是归一化的参考脉冲：解卷积的匹配核。
type Reference struct {
	Shape   []float64 // 归一化形状（峰值 = 1.0）
	WidthNs int64     // 半高宽（纳秒），由调用方按采样率换算
}

// ExtractReference 从去基线波形中、以孤立峰位置为中心提取参考脉冲形状，
// 并按峰值归一化。提取窗口半径 radius 个样本（超出边界截断）。
func ExtractReference(wave []float64, peakPos, radius int) []float64 {
	if len(wave) == 0 || radius <= 0 {
		return nil
	}
	start := peakPos - radius
	if start < 0 {
		start = 0
	}
	end := peakPos + radius + 1
	if end > len(wave) {
		end = len(wave)
	}
	if start >= end {
		return nil
	}
	shape := make([]float64, end-start)
	peak := 0.0
	for i := start; i < end; i++ {
		shape[i-start] = wave[i]
		if wave[i] > peak {
			peak = wave[i]
		}
	}
	if peak <= 0 {
		return nil
	}
	for i := range shape {
		shape[i] /= peak
	}
	return shape
}

// ExtractReferenceForPeak is the explicit reference-extraction entry point
// used after the caller has verified that the peak is isolated.
func ExtractReferenceForPeak(wave []float64, peakPos, radius int) []float64 {
	return ExtractReference(wave, peakPos, radius)
}

// Normalize 把任意形状按峰值归一化到 1.0。
func Normalize(shape []float64) []float64 {
	if len(shape) == 0 {
		return nil
	}
	peak := 0.0
	for _, v := range shape {
		if v > peak {
			peak = v
		}
	}
	if peak <= 0 {
		return nil
	}
	out := make([]float64, len(shape))
	for i, v := range shape {
		out[i] = v / peak
	}
	return out
}

// HalfWidthSamples 计算归一化形状的半高宽（样本数，幅度 >= 0.5 的连续区间宽度）。
func HalfWidthSamples(shape []float64) int {
	if len(shape) == 0 {
		return 0
	}
	left, right := -1, -1
	for i, v := range shape {
		if v >= 0.5 {
			if left == -1 {
				left = i
			}
			right = i
		}
	}
	if left == -1 {
		return 0
	}
	return right - left + 1
}

// WidthNsAtSampleRate 把样本宽度换算为纳秒。
func WidthNsAtSampleRate(widthSamples int, sampleRateHz float64) int64 {
	if sampleRateHz <= 0 {
		return 0
	}
	// 每个样本的时长 = 1/采样率 秒 = 1e9/采样率 纳秒。
	return int64(float64(widthSamples) * 1e9 / sampleRateHz)
}
