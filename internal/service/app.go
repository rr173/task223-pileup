// Package service 编排层：组装各业务模块并暴露给 httpapi 与 main。
package service

import (
	"sync"

	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

// App 应用编排根：聚合全部模块服务。
type App struct {
	db *store.DB

	Runs      *RunService
	Windows   *WindowService
	Deconv    *DeconvService
	Snapshots *SnapshotService

	stats *store.StatsStore

	mu sync.Mutex // 运行状态流转与快照发布串行
}

// New 组装应用服务。
func New(db *store.DB) (*App, error) {
	runStore := store.NewRunStore(db)
	windowStore := store.NewWindowStore(db)
	pulseStore := store.NewPulseStore(db)
	deadZoneStore := store.NewDeadZoneStore(db)
	baselineStore := store.NewBaselineStore(db)
	snapshotStore := store.NewSnapshotStore(db)
	stats := store.NewStatsStore(db)

	app := &App{
		db:    db,
		stats: stats,
	}
	app.Runs = NewRunService(runStore)
	app.Windows = NewWindowService(windowStore, runStore)
	app.Deconv = NewDeconvService(runStore, windowStore, pulseStore, deadZoneStore, baselineStore)
	app.Snapshots = NewSnapshotService(snapshotStore, pulseStore, deadZoneStore, runStore, windowStore)
	return app, nil
}

// DB 暴露底层连接（自检用）。
func (a *App) DB() *store.DB { return a.db }

// Stats 返回全局统计快照。
func (a *App) Stats() (*model.StatSummary, error) { return a.stats.Global() }
