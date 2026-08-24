package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task223-pileup/internal/model"
)

// BaselineStore 基线参数表 + 参考脉冲表 CRUD。
type BaselineStore struct{ db *DB }

// NewBaselineStore 构造基线/参考脉冲存储。
func NewBaselineStore(db *DB) *BaselineStore { return &BaselineStore{db: db} }

// --- 基线参数 ---

const baselineCols = `id, run_id, level, drift_slope, noise_floor, window_count, locked, created_at`

func scanBaseline(sc scanner) (*model.BaselineRecord, error) {
	var b model.BaselineRecord
	var locked int
	var created string
	if err := sc.Scan(&b.ID, &b.RunID, &b.Level, &b.DriftSlope, &b.NoiseFloor, &b.WindowCount, &locked, &created); err != nil {
		return nil, err
	}
	b.Locked = locked != 0
	var err error
	if b.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &b, nil
}

// UpsertBaseline 写入（或替换）运行基线，唯一键为 run_id。
func (s *BaselineStore) UpsertBaseline(b *model.BaselineRecord) error {
	locked := 0
	if b.Locked {
		locked = 1
	}
	_, err := s.db.SQL().Exec(
		`INSERT INTO baselines (id, run_id, level, drift_slope, noise_floor, window_count, locked, created_at)
		 VALUES (?,?,?,?,?,?,?,?)
		 ON CONFLICT(run_id) DO UPDATE SET level=excluded.level, drift_slope=excluded.drift_slope,
		   noise_floor=excluded.noise_floor, window_count=excluded.window_count, locked=excluded.locked`,
		b.ID, b.RunID, b.Level, b.DriftSlope, b.NoiseFloor, b.WindowCount, locked, ts(b.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert baseline: %w", err)
	}
	return nil
}

// GetBaseline 读取运行基线。
func (s *BaselineStore) GetBaseline(runID string) (*model.BaselineRecord, error) {
	row := s.db.SQL().QueryRow(`SELECT `+baselineCols+` FROM baselines WHERE run_id = ?`, runID)
	b, err := scanBaseline(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// LockBaseline 锁定运行基线。
func (s *BaselineStore) LockBaseline(runID string) error {
	_, err := s.db.SQL().Exec(`UPDATE baselines SET locked = 1 WHERE run_id = ?`, runID)
	if err != nil {
		return fmt.Errorf("lock baseline: %w", err)
	}
	return nil
}

// --- 参考脉冲 ---

const refPulseCols = `id, run_id, amplitude, width_ns, shape, source_window, locked_at, created_at`

func scanRefPulse(sc scanner) (*model.ReferencePulse, error) {
	var p model.ReferencePulse
	var locked, created string
	if err := sc.Scan(&p.ID, &p.RunID, &p.Amplitude, &p.WidthNs, &p.Shape, &p.SourceWindow, &locked, &created); err != nil {
		return nil, err
	}
	var err error
	if p.LockedAt, err = parseTS(locked); err != nil {
		return nil, err
	}
	if p.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertReferencePulse 写入（或替换）运行参考脉冲，唯一键为 run_id。
func (s *BaselineStore) UpsertReferencePulse(p *model.ReferencePulse) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO reference_pulses (id, run_id, amplitude, width_ns, shape, source_window, locked_at, created_at)
		 VALUES (?,?,?,?,?,?,?,?)
		 ON CONFLICT(run_id) DO UPDATE SET amplitude=excluded.amplitude, width_ns=excluded.width_ns,
		   shape=excluded.shape, source_window=excluded.source_window, locked_at=excluded.locked_at`,
		p.ID, p.RunID, p.Amplitude, p.WidthNs, p.Shape, p.SourceWindow, ts(p.LockedAt), ts(p.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert reference pulse: %w", err)
	}
	return nil
}

// GetReferencePulse 读取运行参考脉冲。
func (s *BaselineStore) GetReferencePulse(runID string) (*model.ReferencePulse, error) {
	row := s.db.SQL().QueryRow(`SELECT `+refPulseCols+` FROM reference_pulses WHERE run_id = ?`, runID)
	p, err := scanRefPulse(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}
