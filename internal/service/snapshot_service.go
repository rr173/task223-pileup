package service

import (
	"encoding/json"
	"fmt"
	"sync"
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
	mu            *sync.Mutex
}

// NewSnapshotService 构造快照服务。
func NewSnapshotService(s *store.SnapshotStore, p *store.PulseStore, d *store.DeadZoneStore, r *store.RunStore, w *store.WindowStore, mu *sync.Mutex) *SnapshotService {
	return &SnapshotService{
		store:         s,
		pulseStore:    p,
		deadZoneStore: d,
		runStore:      r,
		windowStore:   w,
		builder:       snapshot.NewBuilder(),
		mu:            mu,
	}
}

// Publish 发布一次计数快照：汇总计数、构建不可变快照、封存运行、
// 并把旧快照标记为替代版本。
func (s *SnapshotService) Publish(runID string) (*model.CountSnapshot, error) {
	if s.mu != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	run, err := s.runStore.Get(runID)
	if err != nil {
		return nil, err
	}
	if run.Status != model.RunCompleted {
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

	// 版本号。
	version, err := s.store.NextVersion(runID)
	if err != nil {
		return nil, err
	}

	// 构建快照并发布。
	sn := s.builder.Build(runID, version, sum, string(pulsesJSON))
	sn.Status = model.SnapshotPublished
	now := time.Now().UTC()
	sn.PublishedAt = &now

	// 旧已发布快照 → superseded。
	if err := s.store.MarkSuperseded(runID); err != nil {
		return nil, err
	}
	if err := s.store.Create(sn); err != nil {
		return nil, err
	}

	// 封存运行。
	if _, err := s.runStore.UpdateStatus(runID, model.RunCompleted, model.RunSealed); err != nil {
		return nil, err
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
