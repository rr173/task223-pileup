package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task223-pileup/internal/model"
)

// WindowStore 波形窗口表 CRUD。
type WindowStore struct{ db *DB }

// NewWindowStore 构造窗口存储。
func NewWindowStore(db *DB) *WindowStore { return &WindowStore{db: db} }

const windowCols = `id, run_id, trigger_index, start_time_ns, duration_ns, samples, baseline_level, peak_amplitude, saturated, status, fingerprint, created_at`

func scanWindow(sc scanner) (*model.WaveformWindow, error) {
	var w model.WaveformWindow
	var saturated int
	var created string
	if err := sc.Scan(&w.ID, &w.RunID, &w.TriggerIndex, &w.StartTimeNs, &w.DurationNs,
		&w.Samples, &w.BaselineLevel, &w.PeakAmplitude, &saturated, &w.Status, &w.Fingerprint, &created); err != nil {
		return nil, err
	}
	w.Saturated = saturated != 0
	var err error
	if w.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &w, nil
}

// Create 插入窗口。触发序号冲突时返回 ErrDuplicate。
func (s *WindowStore) Create(w *model.WaveformWindow) error {
	sat := 0
	if w.Saturated {
		sat = 1
	}
	_, err := s.db.SQL().Exec(
		`INSERT INTO windows (id, run_id, trigger_index, start_time_ns, duration_ns, samples, baseline_level, peak_amplitude, saturated, status, fingerprint, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		w.ID, w.RunID, w.TriggerIndex, w.StartTimeNs, w.DurationNs, w.Samples, w.BaselineLevel,
		w.PeakAmplitude, sat, w.Status, w.Fingerprint, ts(w.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("insert window: %w", err)
	}
	return nil
}

// Get 按 ID 读取窗口。
func (s *WindowStore) Get(id string) (*model.WaveformWindow, error) {
	row := s.db.SQL().QueryRow(`SELECT `+windowCols+` FROM windows WHERE id = ?`, id)
	w, err := scanWindow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return w, nil
}

// ListByRun 返回某运行的窗口（按触发序号升序）。
func (s *WindowStore) ListByRun(runID string) ([]model.WaveformWindow, error) {
	rows, err := s.db.SQL().Query(`SELECT `+windowCols+` FROM windows WHERE run_id = ? ORDER BY trigger_index ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WaveformWindow
	for rows.Next() {
		w, err := scanWindow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

// LatestTrigger 返回某运行的最大触发序号（无窗口时返回 0）。
func (s *WindowStore) LatestTrigger(runID string) (int64, error) {
	var n int64
	err := s.db.SQL().QueryRow(`SELECT COALESCE(MAX(trigger_index),0) FROM windows WHERE run_id = ?`, runID).Scan(&n)
	return n, err
}

// UpdateClassify 更新窗口分类状态（baseline_level / peak_amplitude / saturated / status）。
func (s *WindowStore) UpdateClassify(id string, baseline, peak float64, saturated bool, status string) error {
	sat := 0
	if saturated {
		sat = 1
	}
	_, err := s.db.SQL().Exec(
		`UPDATE windows SET baseline_level = ?, peak_amplitude = ?, saturated = ?, status = ? WHERE id = ?`,
		baseline, peak, sat, status, id,
	)
	if err != nil {
		return fmt.Errorf("update window classify: %w", err)
	}
	return nil
}

// CountSaturated 统计某运行的饱和窗口数。
func (s *WindowStore) CountSaturated(runID string) (int, error) {
	var n int
	err := s.db.SQL().QueryRow(`SELECT COUNT(*) FROM windows WHERE run_id = ? AND status = ?`, runID, model.WindowSaturated).Scan(&n)
	return n, err
}
