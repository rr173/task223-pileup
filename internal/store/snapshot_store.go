package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task223-pileup/internal/model"
)

// SnapshotStore 计数快照表 CRUD。
type SnapshotStore struct{ db *DB }

// NewSnapshotStore 构造快照存储。
func NewSnapshotStore(db *DB) *SnapshotStore { return &SnapshotStore{db: db} }

const snapshotCols = `id, run_id, version, status, total_counts, recovered_counts, unresolved_counts, observed_count_rate, true_count_rate, effective_observation_ns, dead_time_fraction, unrecoverable_zones, pulses_json, summary, created_at, published_at`

func scanSnapshot(sc scanner) (*model.CountSnapshot, error) {
	var s model.CountSnapshot
	var created string
	var published sql.NullString
	if err := sc.Scan(&s.ID, &s.RunID, &s.Version, &s.Status, &s.TotalCounts, &s.RecoveredCounts,
		&s.UnresolvedCounts, &s.ObservedCountRate, &s.TrueCountRate, &s.EffectiveObservationNs,
		&s.DeadTimeFraction, &s.UnrecoverableZones, &s.PulsesJSON, &s.Summary, &created, &published); err != nil {
		return nil, err
	}
	var err error
	if s.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	if published.Valid {
		t, err := parseTS(published.String)
		if err != nil {
			return nil, err
		}
		s.PublishedAt = &t
	}
	return &s, nil
}

// NextVersion 返回某运行下一个快照版本号（无快照时为 1）。
func (s *SnapshotStore) NextVersion(runID string) (int, error) {
	var n int
	err := s.db.SQL().QueryRow(`SELECT COALESCE(MAX(version),0) FROM snapshots WHERE run_id = ?`, runID).Scan(&n)
	return n + 1, err
}

// NextVersionTx 在给定事务内读取下一个快照版本号。
// 版本号的读与随后的插入放在同一事务内紧邻执行，避免跨请求读到同一版本号
// 而争抢 (run_id, version) 唯一约束。
func (s *SnapshotStore) NextVersionTx(tx *sql.Tx, runID string) (int, error) {
	var n int
	err := tx.QueryRow(`SELECT COALESCE(MAX(version),0) FROM snapshots WHERE run_id = ?`, runID).Scan(&n)
	return n + 1, err
}

// Create 插入快照。
func (s *SnapshotStore) Create(sn *model.CountSnapshot) error {
	var published any
	if sn.PublishedAt != nil {
		published = ts(*sn.PublishedAt)
	}
	_, err := s.db.SQL().Exec(
		`INSERT INTO snapshots (id, run_id, version, status, total_counts, recovered_counts, unresolved_counts, observed_count_rate, true_count_rate, effective_observation_ns, dead_time_fraction, unrecoverable_zones, pulses_json, summary, created_at, published_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sn.ID, sn.RunID, sn.Version, sn.Status, sn.TotalCounts, sn.RecoveredCounts, sn.UnresolvedCounts,
		sn.ObservedCountRate, sn.TrueCountRate, sn.EffectiveObservationNs, sn.DeadTimeFraction,
		sn.UnrecoverableZones, sn.PulsesJSON, sn.Summary, ts(sn.CreatedAt), published,
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

// CreateTx 在给定事务内插入快照。与封存决策共享同一事务，
// 确保封存与快照写入原子提交。
func (s *SnapshotStore) CreateTx(tx *sql.Tx, sn *model.CountSnapshot) error {
	var published any
	if sn.PublishedAt != nil {
		published = ts(*sn.PublishedAt)
	}
	_, err := tx.Exec(
		`INSERT INTO snapshots (id, run_id, version, status, total_counts, recovered_counts, unresolved_counts, observed_count_rate, true_count_rate, effective_observation_ns, dead_time_fraction, unrecoverable_zones, pulses_json, summary, created_at, published_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sn.ID, sn.RunID, sn.Version, sn.Status, sn.TotalCounts, sn.RecoveredCounts, sn.UnresolvedCounts,
		sn.ObservedCountRate, sn.TrueCountRate, sn.EffectiveObservationNs, sn.DeadTimeFraction,
		sn.UnrecoverableZones, sn.PulsesJSON, sn.Summary, ts(sn.CreatedAt), published,
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

// Get 按 ID 读取快照。
func (s *SnapshotStore) Get(id string) (*model.CountSnapshot, error) {
	row := s.db.SQL().QueryRow(`SELECT `+snapshotCols+` FROM snapshots WHERE id = ?`, id)
	sn, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return sn, nil
}

// ListByRun 返回某运行的快照（按版本升序）。
func (s *SnapshotStore) ListByRun(runID string) ([]model.CountSnapshot, error) {
	rows, err := s.db.SQL().Query(`SELECT `+snapshotCols+` FROM snapshots WHERE run_id = ? ORDER BY version ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.CountSnapshot
	for rows.Next() {
		sn, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sn)
	}
	return out, rows.Err()
}

// MarkSuperseded 把某运行已发布快照标记为替代（仅当状态为 published）。
func (s *SnapshotStore) MarkSuperseded(runID string) error {
	_, err := s.db.SQL().Exec(
		`UPDATE snapshots SET status = ? WHERE run_id = ? AND status = ?`,
		model.SnapshotSuperseded, runID, model.SnapshotPublished,
	)
	if err != nil {
		return fmt.Errorf("supersede snapshots: %w", err)
	}
	return nil
}

// MarkSupersededTx 在给定事务内把已发布快照标记为替代，与封存/写入共享事务。
func (s *SnapshotStore) MarkSupersededTx(tx *sql.Tx, runID string) error {
	_, err := tx.Exec(
		`UPDATE snapshots SET status = ? WHERE run_id = ? AND status = ?`,
		model.SnapshotSuperseded, runID, model.SnapshotPublished,
	)
	if err != nil {
		return fmt.Errorf("supersede snapshots: %w", err)
	}
	return nil
}
