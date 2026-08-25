package deconv

// Constraints 描述解卷积必须满足的约束，用于校验恢复结果是否可信。
type Constraints struct {
	MinAmplitude  float64 // 最小恢复幅度（低于视为噪声，剔除）
	MinSeparation int     // 最小脉冲间隔（样本）
	MaxResidual   float64 // 最大允许残差占比（超过则不可分离）
}

// DefaultConstraints 返回默认约束：幅度 0.05、间隔 4 样本、残差 0.35。
func DefaultConstraints() Constraints {
	return Constraints{MinAmplitude: 0.05, MinSeparation: 4, MaxResidual: 0.35}
}

// Filter 先剔除幅度低于 MinAmplitude 的伪脉冲，再在剩余候选中执行最小间隔约束。
//
// 解卷积内部以较宽松的间隔抑制重复检出（允许候选相距约 2~3 样本），但约束要求
// 的最小可分辨间隔（MinSeparation，默认 4 样本）更严。因此幅度过滤之后仍需对
// 间距小于 MinSeparation 的冲突候选去重：保留幅度更大（更可信）的那个，丢弃
// 与之过近的重复候选；已满足间隔要求的脉冲与所有候选间距均 >= MinSeparation，
// 永远不会被判定为冲突，因而不会受影响。结果按位置升序返回。
func (c Constraints) Filter(pulses []RecoveredPulse) []RecoveredPulse {
	// 1. 幅度过滤：剔除低于最小幅度的伪脉冲。
	kept := make([]RecoveredPulse, 0, len(pulses))
	for _, p := range pulses {
		if p.Amplitude >= c.MinAmplitude {
			kept = append(kept, p)
		}
	}
	// 间隔无效或候选不足时无需去重，仅按位置排序返回。
	if c.MinSeparation <= 1 || len(kept) <= 1 {
		sortPulses(kept)
		return kept
	}

	// 2. 按幅度降序贪心选择：优先接纳更可信（幅度更大）的候选，仅当其与所有
	// 已选脉冲的间距均 >= MinSeparation 时才保留；否则视为与前一个候选重复而丢弃。
	// 幅度相同者按位置升序取舍，保证结果确定性。
	sortPulsesByAmpDesc(kept)
	selected := make([]RecoveredPulse, 0, len(kept))
	for _, p := range kept {
		conflict := false
		for _, s := range selected {
			if absInt(p.Position-s.Position) < c.MinSeparation {
				conflict = true
				break
			}
		}
		if !conflict {
			selected = append(selected, p)
		}
	}

	// 3. 按位置升序返回，便于上层按时间序消费。
	sortPulses(selected)
	return selected
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
