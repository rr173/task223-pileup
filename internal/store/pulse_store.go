package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task223-pileup/internal/model"
)

// PulseStore 脉冲事件表 CRUD。
type PulseStore struct{ db *DB }

// NewPulseStore 构造脉冲存储。
func NewPulseStore(db *DB) *PulseStore { return &PulseStore{db: db} }

const pulseCols = `id, run_id, window_id, arrival_time_ns, amplitude, group_index, status, residual_ratio, confidence, created_at, updated_at`

func scanPulse(sc scanner) (*model.PulseEvent, error) {
	var p model.PulseEvent
	var created, updated string
	if err := sc.Scan(&p.ID, &p.RunID, &p.WindowID, &p.ArrivalTimeNs, &p.Amplitude,
		&p.GroupIndex, &p.Status, &p.ResidualRatio, &p.Confidence, &created, &updated); err != nil {
		return nil, err
	}
	var err error
	if p.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	if p.UpdatedAt, err = parseTS(updated); err != nil {
		return nil, err
	}
	return &p, nil
}

// Create 插入脉冲。
func (s *PulseStore) Create(p *model.PulseEvent) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO pulses (id, run_id, window_id, arrival_time_ns, amplitude, group_index, status, residual_ratio, confidence, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.RunID, p.WindowID, p.ArrivalTimeNs, p.Amplitude, p.GroupIndex,
		p.Status, p.ResidualRatio, p.Confidence, ts(p.CreatedAt), ts(p.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert pulse: %w", err)
	}
	return nil
}

// Get 按 ID 读取脉冲。
func (s *PulseStore) Get(id string) (*model.PulseEvent, error) {
	row := s.db.SQL().QueryRow(`SELECT `+pulseCols+` FROM pulses WHERE id = ?`, id)
	p, err := scanPulse(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ListByRun 返回某运行的脉冲（按到达时间升序）。
func (s *PulseStore) ListByRun(runID string) ([]model.PulseEvent, error) {
	rows, err := s.db.SQL().Query(`SELECT `+pulseCols+` FROM pulses WHERE run_id = ? ORDER BY arrival_time_ns ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PulseEvent
	for rows.Next() {
		p, err := scanPulse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// ListByWindow 返回某窗口的脉冲（按到达时间升序）。
func (s *PulseStore) ListByWindow(windowID string) ([]model.PulseEvent, error) {
	rows, err := s.db.SQL().Query(`SELECT `+pulseCols+` FROM pulses WHERE window_id = ? ORDER BY arrival_time_ns ASC`, windowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PulseEvent
	for rows.Next() {
		p, err := scanPulse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// UpdateStatus 更新脉冲状态。
func (s *PulseStore) UpdateStatus(id, next string) error {
	_, err := s.db.SQL().Exec(
		`UPDATE pulses SET status = ?, updated_at = ? WHERE id = ?`,
		next, ts(nowUTC()), id,
	)
	if err != nil {
		return fmt.Errorf("update pulse status: %w", err)
	}
	return nil
}

// DeleteByRun 删除某运行的全部脉冲（解卷积重跑前清理旧结果）。
func (s *PulseStore) DeleteByRun(runID string) error {
	_, err := s.db.SQL().Exec(`DELETE FROM pulses WHERE run_id = ?`, runID)
	if err != nil {
		return fmt.Errorf("delete pulses: %w", err)
	}
	return nil
}

// CountSeparated 统计某运行已分离/已确认的脉冲数。
func (s *PulseStore) CountSeparated(runID string) (int, error) {
	var n int
	err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM pulses WHERE run_id = ? AND status IN (?, ?)`,
		runID, model.PulseSeparated, model.PulseConfirmed).Scan(&n)
	return n, err
}

// CountRecovered 统计由堆积解卷积恢复的脉冲（group_index > 0）。
func (s *PulseStore) CountRecovered(runID string) (int, error) {
	var n int
	err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM pulses WHERE run_id = ? AND status IN (?, ?)`,
		runID, model.PulseSeparated, model.PulseConfirmed).Scan(&n)
	return n, err
}

// CountInseparable 统计某运行不可分离的脉冲数。
func (s *PulseStore) CountInseparable(runID string) (int, error) {
	var n int
	err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM pulses WHERE run_id = ? AND status = ?`,
		runID, model.PulseInseparable).Scan(&n)
	return n, err
}
