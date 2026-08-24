package deconv

// Constraints 描述解卷积必须满足的约束，用于校验恢复结果是否可信。
type Constraints struct {
	MinAmplitude float64 // 最小恢复幅度（低于视为噪声，剔除）
	MinSeparation int    // 最小脉冲间隔（样本）
	MaxResidual  float64 // 最大允许残差占比（超过则不可分离）
}

// DefaultConstraints 返回默认约束：幅度 0.05、间隔 4 样本、残差 0.35。
func DefaultConstraints() Constraints {
	return Constraints{MinAmplitude: 0.05, MinSeparation: 4, MaxResidual: 0.35}
}

// Filter 剔除幅度低于 MinAmplitude 的伪脉冲，返回满足约束的脉冲。
func (c Constraints) Filter(pulses []RecoveredPulse) []RecoveredPulse {
	out := make([]RecoveredPulse, 0, len(pulses))
	for _, p := range pulses {
		if p.Amplitude >= c.MinAmplitude {
			out = append(out, p)
		}
	}
	return out
}

// Separable 判断一次解卷积结果是否「可分离」：
// 过滤后仍有脉冲，且残差占比不超过上限。
func (c Constraints) Separable(result Result) bool {
	filtered := c.Filter(result.Pulses)
	return len(filtered) > 0 && result.Residual <= c.MaxResidual
}

// NonNegative 校验脉冲幅度是否全部非负。
func NonNegative(pulses []RecoveredPulse) bool {
	for _, p := range pulses {
		if p.Amplitude < 0 {
			return false
		}
	}
	return true
}
