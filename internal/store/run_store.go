package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task223-pileup/internal/model"
)

// RunStore 运行表 CRUD。
type RunStore struct{ db *DB }

// NewRunStore 构造运行存储。
func NewRunStore(db *DB) *RunStore { return &RunStore{db: db} }

const runCols = `id, name, description, detector_type, sample_rate_hz, dead_time_ns, status, fingerprint, created_at, updated_at`

func scanRun(sc scanner) (*model.Run, error) {
	var r model.Run
	var created, updated string
	if err := sc.Scan(&r.ID, &r.Name, &r.Description, &r.DetectorType, &r.SampleRateHz,
		&r.DeadTimeNs, &r.Status, &r.Fingerprint, &created, &updated); err != nil {
		return nil, err
	}
	var err error
	if r.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	if r.UpdatedAt, err = parseTS(updated); err != nil {
		return nil, err
	}
	return &r, nil
}

// Create 插入运行。指纹冲突时返回 ErrDuplicate。
func (s *RunStore) Create(r *model.Run) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO runs (id, name, description, detector_type, sample_rate_hz, dead_time_ns, status, fingerprint, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Name, r.Description, r.DetectorType, r.SampleRateHz, r.DeadTimeNs,
		r.Status, r.Fingerprint, ts(r.CreatedAt), ts(r.UpdatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

// Get 按 ID 读取运行。
func (s *RunStore) Get(id string) (*model.Run, error) {
	row := s.db.SQL().QueryRow(`SELECT `+runCols+` FROM runs WHERE id = ?`, id)
	r, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// List 返回全部运行（按创建时间倒序）。
func (s *RunStore) List() ([]model.Run, error) {
	rows, err := s.db.SQL().Query(`SELECT ` + runCols + ` FROM runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// UpdateStatus 更新运行状态（仅当当前状态匹配 expected，防并发乱序流转）。
func (s *RunStore) UpdateStatus(id, expected, next string) (bool, error) {
	res, err := s.db.SQL().Exec(
		`UPDATE runs SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		next, ts(nowUTC()), id, expected,
	)
	if err != nil {
		return false, fmt.Errorf("update run status: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// CountOpenRuns 统计未封存的运行数。
func (s *RunStore) CountOpenRuns() (int, error) {
	var n int
	err := s.db.SQL().QueryRow(`SELECT COUNT(*) FROM runs WHERE status != ?`, model.RunSealed).Scan(&n)
	return n, err
}
