// Command pileup 辐射探测器脉冲堆积解卷积服务入口。
//
// 支持三个标志：
//   - --addr :8080      监听地址（默认 :8080）
//   - --db ./pileup.db  SQLite 数据库路径（默认 ./pileup.db）
//   - --smoke-test      执行端到端冒烟：真实创建数据、关闭并重开数据库
//     验证持久化与重启恢复，随后以 0 退出码结束。
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"

	"task223-pileup/internal/httpapi"
	"task223-pileup/internal/model"
	"task223-pileup/internal/service"
	"task223-pileup/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "./pileup.db", "SQLite database path")
	smoke := flag.Bool("smoke-test", false, "run end-to-end smoke test and exit")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			fmt.Fprintln(os.Stderr, "SMOKE TEST FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("SMOKE TEST PASSED")
		return
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	app, err := service.New(db)
	if err != nil {
		log.Fatalf("init services: %v", err)
	}
	srv := httpapi.New(app)
	log.Printf("task223-pileup listening on %s (db=%s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

// 冒烟测试的波形参数。
const (
	smokeSampleRate = 5e8 // 500 MHz 采样率（每 2ns 一个样本）
	smokeWindowNs   = 400 // 每窗口时长（纳秒）
	smokeDeadTimeNs = 40  // 死区时间（纳秒）→ 20 样本
	smokeSamples    = 200 // 每窗口样本数（400ns / 2ns）
	smokeWindows    = 12  // 总窗口数
	smokePulseSigma = 4.0 // 高斯脉冲宽度（样本），半高宽约 9.4 样本
)

// addPulse 把一个高斯脉冲叠加到波形上（幅度 amp、中心 center、宽度 sigma）。
func addPulse(samples []float64, center, amp, sigma float64) {
	for i := range samples {
		x := float64(i) - center
		samples[i] += amp * math.Exp(-(x*x)/(2*sigma*sigma))
	}
}

// genIsolated 生成含单个孤立脉冲的窗口。
func genIsolated(rng *rand.Rand, center float64) []float64 {
	s := make([]float64, smokeSamples)
	for i := range s {
		s[i] = 0.02 + rng.NormFloat64()*0.01
	}
	addPulse(s, center, 0.8, smokePulseSigma)
	return s
}

// genPiled 生成含两个重叠脉冲（堆积）的窗口：间距 14 样本，小于死区 20 样本
// 但大于脉冲半高宽，构成可分离的堆积。
func genPiled(rng *rand.Rand, c1, c2 float64) []float64 {
	s := make([]float64, smokeSamples)
	for i := range s {
		s[i] = 0.02 + rng.NormFloat64()*0.01
	}
	addPulse(s, c1, 0.8, smokePulseSigma)
	addPulse(s, c2, 0.6, smokePulseSigma)
	return s
}

// genSaturated 生成饱和窗口：脉冲幅度超出满量程，裁剪为平顶。
func genSaturated(rng *rand.Rand, center float64) []float64 {
	s := make([]float64, smokeSamples)
	for i := range s {
		s[i] = 0.02 + rng.NormFloat64()*0.01
	}
	addPulse(s, center, 5.0, 10.0)
	for i := range s {
		if s[i] > 1.0 {
			s[i] = 1.0
		}
	}
	return s
}

// runSmokeTest 执行端到端冒烟：
//  1. 打开数据库 A，走完整闭环：运行 -> 波形窗口（含孤立/堆积/饱和）-> 解卷积 ->
//     脉冲/死区 -> 确认 -> 发布计数快照（封存）；
//  2. 幂等验证：重复触发序号被跳过；
//  3. 封存后拒绝再接收窗口；
//  4. 关闭数据库 A，重开同一路径数据库 B，验证数据仍在（重启恢复）。
func runSmokeTest(dbPath string) error {
	if dbPath != ":memory:" {
		_ = os.Remove(dbPath)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	app, err := service.New(db)
	if err != nil {
		db.Close()
		return fmt.Errorf("init services: %w", err)
	}

	rng := rand.New(rand.NewSource(42))

	// --- 步骤 1：创建运行 ---
	run, err := app.Runs.Create("NaI 探测器脉冲堆积实验 A", "1 MHz 计数率下的脉冲堆积解卷积", "NaI", smokeSampleRate, smokeDeadTimeNs)
	if err != nil {
		db.Close()
		return fmt.Errorf("create run: %w", err)
	}

	// --- 步骤 2：接收 12 个窗口 ---
	// 布局：0-2 孤立（0 作参考脉冲源），3-5 堆积，6-7 孤立，8 饱和，9-11 孤立。
	for w := 0; w < smokeWindows; w++ {
		var samples []float64
		switch {
		case w >= 3 && w <= 5:
			samples = genPiled(rng, 100, 114)
		case w == 8:
			samples = genSaturated(rng, 100)
		default:
			samples = genIsolated(rng, 100)
		}
		startNs := int64(w) * smokeWindowNs
		if _, err := app.Windows.Ingest(run, int64(w), startNs, smokeWindowNs, samples); err != nil {
			db.Close()
			return fmt.Errorf("ingest w=%d: %w", w, err)
		}
	}

	// --- 步骤 3：幂等验证：重复触发序号被跳过 ---
	res, err := app.Windows.Ingest(run, 5, 5*smokeWindowNs, smokeWindowNs, genIsolated(rng, 100))
	if err != nil {
		db.Close()
		return fmt.Errorf("re-ingest: %w", err)
	}
	if res.Inserted != 0 || res.Duplicate != 1 {
		db.Close()
		return fmt.Errorf("re-ingest should be duplicate, got inserted=%d duplicate=%d", res.Inserted, res.Duplicate)
	}

	// --- 步骤 4：结束接收，进入处理 ---
	run, err = app.Runs.FinishReceiving(run.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("finish receiving: %w", err)
	}
	if run.Status != model.RunProcessing {
		db.Close()
		return fmt.Errorf("run should be processing, got %s", run.Status)
	}

	// --- 步骤 5：解卷积 ---
	deconvRes, err := app.Deconv.Deconvolve(run.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("deconvolve: %w", err)
	}
	if deconvRes.RecoveredPulses == 0 {
		db.Close()
		return fmt.Errorf("no recovered pulses")
	}
	if deconvRes.DeadZones < 1 {
		db.Close()
		return fmt.Errorf("expected at least 1 dead zone (saturated window), got %d", deconvRes.DeadZones)
	}

	// 验证脉冲已持久化。
	pulses, err := app.Deconv.ListPulses(run.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("list pulses: %w", err)
	}
	if len(pulses) == 0 {
		db.Close()
		return fmt.Errorf("no pulses persisted")
	}
	separated := 0
	for _, p := range pulses {
		if p.Status == model.PulseSeparated || p.Status == model.PulseConfirmed {
			separated++
		}
	}
	if separated == 0 {
		db.Close()
		return fmt.Errorf("no separated pulses")
	}

	// 验证死区已持久化（饱和窗口应产生死区）。
	zones, err := app.Deconv.ListDeadZones(run.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("list dead zones: %w", err)
	}
	if len(zones) == 0 {
		db.Close()
		return fmt.Errorf("no dead zones persisted")
	}

	// --- 步骤 6：确认运行 ---
	for _, p := range pulses {
		if p.Status != model.PulseSeparated {
			continue
		}
		if _, err := app.Deconv.ConfirmPulse(p.ID); err != nil {
			db.Close()
			return fmt.Errorf("confirm pulse %s: %w", p.ID, err)
		}
	}
	run, err = app.Runs.Confirm(run.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("confirm run: %w", err)
	}
	if run.Status != model.RunCompleted {
		db.Close()
		return fmt.Errorf("run should be completed, got %s", run.Status)
	}

	// --- 步骤 7：发布计数快照（封存） ---
	snap, err := app.Snapshots.Publish(run.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("publish snapshot: %w", err)
	}
	if snap.Status != model.SnapshotPublished {
		db.Close()
		return fmt.Errorf("snapshot should be published, got %s", snap.Status)
	}
	if snap.TotalCounts <= 0 {
		db.Close()
		return fmt.Errorf("snapshot total counts should be positive, got %d", snap.TotalCounts)
	}

	// --- 步骤 8：封存后拒绝再接收窗口 ---
	sealedRun, err := app.Runs.Get(run.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get sealed run: %w", err)
	}
	if sealedRun.Status != model.RunSealed {
		db.Close()
		return fmt.Errorf("run should be sealed, got %s", sealedRun.Status)
	}
	if _, err := app.Windows.Ingest(sealedRun, 999, 999*smokeWindowNs, smokeWindowNs, genIsolated(rng, 100)); err == nil {
		db.Close()
		return fmt.Errorf("sealed run should reject window ingest")
	}

	// --- 步骤 9：重启恢复 ---
	db.Close()

	db2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("reopen db: %w", err)
	}
	defer db2.Close()
	app2, err := service.New(db2)
	if err != nil {
		return fmt.Errorf("reinit services: %w", err)
	}
	r2, err := app2.Runs.Get(run.ID)
	if err != nil {
		return fmt.Errorf("get run after reopen: %w", err)
	}
	if r2.Status != model.RunSealed {
		return fmt.Errorf("run status after reopen should be sealed, got %s", r2.Status)
	}
	pulses2, err := app2.Deconv.ListPulses(run.ID)
	if err != nil {
		return fmt.Errorf("list pulses after reopen: %w", err)
	}
	if len(pulses2) == 0 {
		return fmt.Errorf("no pulses after reopen")
	}
	snaps2, err := app2.Snapshots.List(run.ID)
	if err != nil {
		return fmt.Errorf("list snapshots after reopen: %w", err)
	}
	if len(snaps2) == 0 || snaps2[0].Status != model.SnapshotPublished {
		return fmt.Errorf("published snapshot missing after reopen")
	}
	return nil
}
