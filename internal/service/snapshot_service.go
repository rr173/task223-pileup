package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"task223-pileup/internal/counting"
	"task223-pileup/internal/model"
	"task223-pileup/internal/snapshot"
	"task223-pileup/internal/store"
)

// SnapshotService 计数快照发布服务。
type SnapshotService struct {
	store         *store.SnapshotStore
	pulseStore    *store.PulseStore
	deadZoneStore *store.DeadZoneStore
	runStore      *store.RunStore
	windowStore   *store.WindowStore
	builder       *snapshot.Builder
}

// NewSnapshotService 构造快照服务。
func NewSnapshotService(s *store.SnapshotStore, p *store.PulseStore, d *store.DeadZoneStore, r *store.RunStore, w *store.WindowStore) *SnapshotService {
	return &SnapshotService{
		store:         s,
		pulseStore:    p,
		deadZoneStore: d,
		runStore:      r,
		windowStore:   w,
		builder:       snapshot.NewBuilder(),
	}
}

// Publish 发布一次计数快照：汇总计数、构建不可变快照、封存运行、
// 并把旧快照标记为替代版本。
//
// 并发协调：把“运行 completed → sealed”的状态推进作为发布的唯一决策点。
// SealIfCompleted 在单事务内用条件更新
// `UPDATE runs SET status='sealed' WHERE id=? AND status='completed'`
// 决定赢家——只有成功推进状态的那个请求才会进入快照写入；其余并发请求
// 看到运行已被封存，直接返回 model.ErrSealed（运行不可再发布），
// 不再尝试争抢版本号、也不会把 (run_id, version) 唯一约束错误暴露出去。
// 封存决策与快照写入在同一事务内原子提交，保留单次发布的快照与封存结果。
func (s *SnapshotService) Publish(runID string) (*model.CountSnapshot, error) {
	run, err := s.runStore.Get(runID)
	if err != nil {
		return nil, err
	}
	if run.Status != model.RunCompleted {
		if run.Status == model.RunSealed {
			return nil, fmt.Errorf("%w: run is sealed, no longer publishable", model.ErrSealed)
		}
		return nil, fmt.Errorf("%w: run is %s, not completed", model.ErrInvalidState, run.Status)
	}
	if pending, err := s.pulseStore.CountReviewPending(runID); err != nil {
		return nil, err
	} else if pending > 0 {
		return nil, fmt.Errorf("%w: %d pulses still need review", model.ErrConflict, pending)
	}

	// 计数汇总。
	separated, err := s.pulseStore.CountSeparated(runID)
	if err != nil {
		return nil, err
	}
	recovered, err := s.pulseStore.CountRecovered(runID)
	if err != nil {
		return nil, err
	}
	unresolved, err := s.pulseStore.CountInseparable(runID)
	if err != nil {
		return nil, err
	}

	// 观察时间与死区。
	totalObsNs, err := s.totalObservationNs(runID)
	if err != nil {
		return nil, err
	}
	deadZoneNs, err := s.deadZoneStore.TotalDuration(runID)
	if err != nil {
		return nil, err
	}
	zones, err := s.deadZoneStore.CountByRun(runID)
	if err != nil {
		return nil, err
	}

	sum := counting.Aggregate(separated, unresolved, recovered, float64(run.DeadTimeNs)/1e9, totalObsNs, deadZoneNs, zones)

	// 脉冲证据快照。
	pulses, err := s.pulseStore.ListByRun(runID)
	if err != nil {
		return nil, err
	}
	pulsesJSON, err := json.Marshal(pulses)
	if err != nil {
		return nil, err
	}

	// 在单事务内封存运行并写入快照。版本号的读与插入紧邻执行于同一事务，
	// 避免跨请求争抢同一版本号。只有赢家提交，输家返回 ErrSealed。
	var sn *model.CountSnapshot
	sealed, err := s.runStore.SealIfCompleted(runID, func(tx *sql.Tx) error {
		version, err := s.store.NextVersionTx(tx, runID)
		if err != nil {
			return err
		}
		built := s.builder.Build(runID, version, sum, string(pulsesJSON))
		built.Status = model.SnapshotPublished
		now := time.Now().UTC()
		built.PublishedAt = &now
		// 旧已发布快照 → superseded。
		if err := s.store.MarkSupersededTx(tx, runID); err != nil {
			return err
		}
		if err := s.store.CreateTx(tx, built); err != nil {
			return err
		}
		sn = built
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !sealed {
		// run 已被另一并发请求先行封存：不可再发布。
		return nil, fmt.Errorf("%w: run is sealed, no longer publishable", model.ErrSealed)
	}
	return sn, nil
}

// List 返回某运行的快照（按版本升序）。
func (s *SnapshotService) List(runID string) ([]model.CountSnapshot, error) {
	return s.store.ListByRun(runID)
}

// Get 读取快照。
func (s *SnapshotService) Get(id string) (*model.CountSnapshot, error) {
	return s.store.Get(id)
}

// totalObservationNs 计算运行的总观察时间（所有窗口时长之和）。
func (s *SnapshotService) totalObservationNs(runID string) (int64, error) {
	run, err := s.runStore.Get(runID)
	if err != nil {
		return 0, err
	}
	windows, err := s.windowStore.ListByRun(runID)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, w := range windows {
		if w.DurationNs > 0 {
			total += w.DurationNs
			continue
		}
		// 兜底：由样本数估算时长。
		samples, err := decodeSamples(w.Samples)
		if err == nil && len(samples) > 0 && run.SampleRateHz > 0 {
			total += int64(float64(len(samples)) * 1e9 / run.SampleRateHz)
		}
	}
	return total, nil
}
