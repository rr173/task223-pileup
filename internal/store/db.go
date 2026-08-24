// Package store 提供基于 SQLite（modernc.org/sqlite，纯 Go 驱动，CGO 无关）的
// 持久化实现：建表迁移与运行/窗口/脉冲/死区/基线/参考脉冲/快照的 CRUD 及统计查询。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Open 打开（或创建）SQLite 数据库并执行迁移。
// 支持 ":memory:" 用于测试与冒烟验证。
func Open(path string) (*DB, error) {
	if path == ":memory:" {
		db, err := sql.Open("sqlite", "file::memory:?cache=shared")
		if err != nil {
			return nil, fmt.Errorf("open memory db: %w", err)
		}
		db.SetMaxOpenConns(1)
		d := &DB{db: db, path: path}
		if err := d.migrate(); err != nil {
			db.Close()
			return nil, err
		}
		return d, nil
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set wal: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable fk: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	d := &DB{db: db, path: path}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

// DB 封装 SQLite 连接。
type DB struct {
	db   *sql.DB
	path string
}

// Close 关闭数据库连接。
func (d *DB) Close() error { return d.db.Close() }

// Path 返回数据库路径（调试用）。
func (d *DB) Path() string { return d.path }

// SQL 暴露底层连接（供 Store 实现使用）。
func (d *DB) SQL() *sql.DB { return d.db }

// migrate 建表：全部业务表 + 唯一约束（防重复）。
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS runs (
			id             TEXT PRIMARY KEY,
			name           TEXT NOT NULL,
			description    TEXT NOT NULL DEFAULT '',
			detector_type  TEXT NOT NULL DEFAULT '',
			sample_rate_hz REAL NOT NULL,
			dead_time_ns   INTEGER NOT NULL,
			status         TEXT NOT NULL,
			fingerprint    TEXT NOT NULL,
			created_at     TEXT NOT NULL,
			updated_at     TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_fingerprint ON runs(fingerprint)`,
		`CREATE TABLE IF NOT EXISTS windows (
			id             TEXT PRIMARY KEY,
			run_id         TEXT NOT NULL REFERENCES runs(id),
			trigger_index  INTEGER NOT NULL,
			start_time_ns  INTEGER NOT NULL,
			duration_ns    INTEGER NOT NULL,
			samples        TEXT NOT NULL DEFAULT '[]',
			baseline_level REAL NOT NULL DEFAULT 0,
			peak_amplitude REAL NOT NULL DEFAULT 0,
			saturated      INTEGER NOT NULL DEFAULT 0,
			status         TEXT NOT NULL,
			fingerprint    TEXT NOT NULL,
			created_at     TEXT NOT NULL,
			UNIQUE(run_id, trigger_index)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_windows_run ON windows(run_id)`,
		`CREATE TABLE IF NOT EXISTS pulses (
			id              TEXT PRIMARY KEY,
			run_id          TEXT NOT NULL REFERENCES runs(id),
			window_id       TEXT NOT NULL REFERENCES windows(id),
			arrival_time_ns INTEGER NOT NULL,
			amplitude       REAL NOT NULL DEFAULT 0,
			group_index     INTEGER NOT NULL DEFAULT 0,
			status          TEXT NOT NULL,
			residual_ratio  REAL NOT NULL DEFAULT 0,
			confidence      REAL NOT NULL DEFAULT 0,
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pulses_run ON pulses(run_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pulses_window ON pulses(window_id)`,
		`CREATE TABLE IF NOT EXISTS dead_zones (
			id            TEXT PRIMARY KEY,
			run_id        TEXT NOT NULL REFERENCES runs(id),
			start_time_ns INTEGER NOT NULL,
			end_time_ns   INTEGER NOT NULL,
			reason        TEXT NOT NULL,
			created_at    TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_deadzones_run ON dead_zones(run_id)`,
		`CREATE TABLE IF NOT EXISTS baselines (
			id           TEXT PRIMARY KEY,
			run_id       TEXT NOT NULL REFERENCES runs(id),
			level        REAL NOT NULL DEFAULT 0,
			drift_slope  REAL NOT NULL DEFAULT 0,
			noise_floor  REAL NOT NULL DEFAULT 0,
			window_count INTEGER NOT NULL DEFAULT 0,
			locked       INTEGER NOT NULL DEFAULT 0,
			created_at   TEXT NOT NULL,
			UNIQUE(run_id)
		)`,
		`CREATE TABLE IF NOT EXISTS reference_pulses (
			id            TEXT PRIMARY KEY,
			run_id        TEXT NOT NULL REFERENCES runs(id),
			amplitude     REAL NOT NULL DEFAULT 1,
			width_ns      INTEGER NOT NULL DEFAULT 0,
			shape         TEXT NOT NULL DEFAULT '[]',
			source_window TEXT NOT NULL DEFAULT '',
			locked_at     TEXT NOT NULL,
			created_at    TEXT NOT NULL,
			UNIQUE(run_id)
		)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id                     TEXT PRIMARY KEY,
			run_id                 TEXT NOT NULL REFERENCES runs(id),
			version                INTEGER NOT NULL,
			status                 TEXT NOT NULL,
			total_counts           INTEGER NOT NULL DEFAULT 0,
			recovered_counts       INTEGER NOT NULL DEFAULT 0,
			unresolved_counts      INTEGER NOT NULL DEFAULT 0,
			observed_count_rate    REAL NOT NULL DEFAULT 0,
			true_count_rate        REAL NOT NULL DEFAULT 0,
			effective_observation_ns INTEGER NOT NULL DEFAULT 0,
			dead_time_fraction     REAL NOT NULL DEFAULT 0,
			unrecoverable_zones    INTEGER NOT NULL DEFAULT 0,
			pulses_json            TEXT NOT NULL DEFAULT '[]',
			summary                TEXT NOT NULL DEFAULT '',
			created_at             TEXT NOT NULL,
			published_at           TEXT,
			UNIQUE(run_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_run ON snapshots(run_id)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, firstWords(s, 10))
		}
	}
	return nil
}

func firstWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, " ")
}

func nowUTC() time.Time { return time.Now().UTC() }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

type scanner interface{ Scan(dest ...any) error }
