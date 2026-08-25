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

// SealIfCompleted 在单事务内把运行从 completed 推进到 sealed，
// 并在成功推进后于同一事务内执行 fn（fn 负责写入封存结果，如计数快照）。
//
// 作为发布并发协调的唯一决策点：条件更新
// `UPDATE runs SET status='sealed' WHERE id=? AND status='completed'`
// 是原子的——只有成功推进状态的那一个发布者会调用 fn 并提交；
// 其余并发请求影响 0 行（run 已被赢家先行封存），不调用 fn，返回 (false, nil)。
// 调用方据此区分“已被另一请求封存”与“自己成功发布”。
//
// 事务保证封存决策与 fn 的写入要么整体提交、要么整体回滚，
// 不会留下“run 已 sealed 但无快照”的脏状态。
//
// 返回值：
//   - (true, nil)  成功封存且 fn 已提交；
//   - (false, nil) run 已不在 completed（通常已被另一请求封存），fn 未执行；
//   - (false, err) 事务内错误，已回滚。
func (s *RunStore) SealIfCompleted(runID string, fn func(*sql.Tx) error) (bool, error) {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return false, fmt.Errorf("begin seal tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.Exec(
		`UPDATE runs SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		model.RunSealed, ts(nowUTC()), runID, model.RunCompleted,
	)
	if err != nil {
		return false, fmt.Errorf("seal run: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// run 已不在 completed：被另一并发发布者先行封存。
		return false, nil
	}

	if err := fn(tx); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit seal tx: %w", err)
	}
	committed = true
	return true, nil
}

// CountOpenRuns 统计未封存的运行数。
func (s *RunStore) CountOpenRuns() (int, error) {
	var n int
	err := s.db.SQL().QueryRow(`SELECT COUNT(*) FROM runs WHERE status != ?`, model.RunSealed).Scan(&n)
	return n, err
}

// IsSealed reports whether a run is in its immutable terminal state.
func (s *RunStore) IsSealed(id string) (bool, error) {
	var status string
	if err := s.db.SQL().QueryRow(`SELECT status FROM runs WHERE id = ?`, id).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, model.ErrNotFound
		}
		return false, err
	}
	return status == model.RunSealed, nil
}
