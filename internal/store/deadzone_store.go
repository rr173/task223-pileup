package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task223-pileup/internal/model"
)

// DeadZoneStore 死区/不可恢复区表 CRUD。
type DeadZoneStore struct{ db *DB }

// NewDeadZoneStore 构造死区存储。
func NewDeadZoneStore(db *DB) *DeadZoneStore { return &DeadZoneStore{db: db} }

const deadZoneCols = `id, run_id, start_time_ns, end_time_ns, reason, created_at`

func scanDeadZone(sc scanner) (*model.DeadZone, error) {
	var z model.DeadZone
	var created string
	if err := sc.Scan(&z.ID, &z.RunID, &z.StartTimeNs, &z.EndTimeNs, &z.Reason, &created); err != nil {
		return nil, err
	}
	var err error
	if z.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &z, nil
}

// Create 插入死区。
func (s *DeadZoneStore) Create(z *model.DeadZone) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO dead_zones (id, run_id, start_time_ns, end_time_ns, reason, created_at)
		 VALUES (?,?,?,?,?,?)`,
		z.ID, z.RunID, z.StartTimeNs, z.EndTimeNs, z.Reason, ts(z.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert dead zone: %w", err)
	}
	return nil
}

// ListByRun 返回某运行的死区（按起始时间升序）。
func (s *DeadZoneStore) ListByRun(runID string) ([]model.DeadZone, error) {
	rows, err := s.db.SQL().Query(`SELECT `+deadZoneCols+` FROM dead_zones WHERE run_id = ? ORDER BY start_time_ns ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DeadZone
	for rows.Next() {
		z, err := scanDeadZone(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *z)
	}
	return out, rows.Err()
}

// DeleteByRun 删除某运行的全部死区（解卷积重跑前清理旧结果）。
func (s *DeadZoneStore) DeleteByRun(runID string) error {
	_, err := s.db.SQL().Exec(`DELETE FROM dead_zones WHERE run_id = ?`, runID)
	if err != nil {
		return fmt.Errorf("delete dead zones: %w", err)
	}
	return nil
}

// CountByRun 统计某运行的死区数。
func (s *DeadZoneStore) CountByRun(runID string) (int, error) {
	var n int
	err := s.db.SQL().QueryRow(`SELECT COUNT(*) FROM dead_zones WHERE run_id = ?`, runID).Scan(&n)
	return n, err
}

// TotalDuration 返回某运行全部死区总时长（纳秒）。
func (s *DeadZoneStore) TotalDuration(runID string) (int64, error) {
	var n int64
	err := s.db.SQL().QueryRow(
		`SELECT COALESCE(SUM(end_time_ns - start_time_ns), 0) FROM dead_zones WHERE run_id = ?`, runID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, nil
}
