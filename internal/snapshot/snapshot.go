// Package snapshot 负责计数快照的构建与汇总文本渲染。
// 快照把一次运行的计数汇总、脉冲证据与真实计数率冻结为不可变记录，
// 发布后可被更高版本的快照替代。
package snapshot

import (
	"fmt"
	"time"

	"task223-pileup/internal/counting"
	"task223-pileup/internal/model"
)

// Builder 计数快照构建器。
type Builder struct{}

// NewBuilder 构造快照构建器。
func NewBuilder() *Builder { return &Builder{} }

// Build 从计数汇总与脉冲证据组装一份草稿快照。
//
// 参数：
//   - runID：运行 ID。
//   - version：快照版本号。
//   - sum：计数汇总结果。
//   - pulsesJSON：脉冲证据快照（已序列化的 JSON 数组）。
func (b *Builder) Build(runID string, version int, sum counting.Summary, pulsesJSON string) *model.CountSnapshot {
	now := time.Now().UTC()
	return &model.CountSnapshot{
		ID:                     fmt.Sprintf("snap-%s-%d", runID, version),
		RunID:                  runID,
		Version:                version,
		Status:                 model.SnapshotDraft,
		TotalCounts:            sum.TotalCounts,
		RecoveredCounts:        sum.RecoveredCounts,
		UnresolvedCounts:       sum.UnresolvedCounts,
		ObservedCountRate:      sum.ObservedCountRate,
		TrueCountRate:          sum.TrueCountRate,
		EffectiveObservationNs: sum.EffectiveObservationNs,
		DeadTimeFraction:       sum.DeadTimeFraction,
		UnrecoverableZones:     sum.UnrecoverableZones,
		PulsesJSON:             pulsesJSON,
		Summary:                b.Summarize(sum),
		CreatedAt:              now,
	}
}

// Summarize 把计数汇总渲染为人类可读文本。
func (b *Builder) Summarize(sum counting.Summary) string {
	return fmt.Sprintf(
		"total=%d recovered=%d unresolved=%d observed=%.2f cps true=%.2f cps dead_time=%.1f%% zones=%d",
		sum.TotalCounts, sum.RecoveredCounts, sum.UnresolvedCounts,
		sum.ObservedCountRate, sum.TrueCountRate, sum.DeadTimeFraction*100, sum.UnrecoverableZones,
	)
}
