package store

import "task223-pileup/internal/model"

// StatsStore 全局统计查询。
type StatsStore struct{ db *DB }

// NewStatsStore 构造统计存储。
func NewStatsStore(db *DB) *StatsStore { return &StatsStore{db: db} }

// Global 返回全局统计快照。
func (s *StatsStore) Global() (*model.StatSummary, error) {
	sum := &model.StatSummary{}
	queries := []struct {
		dst *int
		sql string
	}{
		{&sum.Runs, `SELECT COUNT(*) FROM runs`},
		{&sum.Windows, `SELECT COUNT(*) FROM windows`},
		{&sum.Pulses, `SELECT COUNT(*) FROM pulses`},
		{&sum.Snapshots, `SELECT COUNT(*) FROM snapshots`},
		{&sum.DeadZones, `SELECT COUNT(*) FROM dead_zones`},
		{&sum.OpenRuns, `SELECT COUNT(*) FROM runs WHERE status != 'sealed'`},
	}
	for _, q := range queries {
		if err := s.db.SQL().QueryRow(q.sql).Scan(q.dst); err != nil {
			return nil, err
		}
	}
	return sum, nil
}
