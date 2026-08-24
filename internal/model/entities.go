// Package model 定义辐射探测器脉冲堆积解卷积服务的领域实体、
// 状态常量与领域错误。实体为纯数据载体，不含持久化与 HTTP 逻辑，
// 业务规则由 baseline/detector/deconv/deadzone/counting/snapshot 各业务包承载。
package model

import "time"

// 采集运行状态机：
// receiving(接收中) -> processing(处理中) -> pending_review(待复核) -> completed(已完成) -> sealed(封存)。
// 封存为单向终态，封存后不可再写入窗口、重新解卷积或覆盖快照。
const (
	RunReceiving     = "receiving"      // 接收中：正在接收波形窗口
	RunProcessing    = "processing"     // 处理中：已停止接收，正在估计基线/识别堆积/解卷积
	RunPendingReview = "pending_review" // 待复核：分解结果待实验人员复核
	RunCompleted     = "completed"      // 已完成：脉冲已确认，计数快照可发布
	RunSealed        = "sealed"         // 封存：快照已发布，冻结为不可变
)

// 波形窗口状态机：
// raw(原始) -> decomposable(可分解)/piled(堆积)/saturated(饱和)/duplicate(重复)。
// 饱和窗口无法伪造分解结果；duplicate 由触发序号幂等键拦截。
const (
	WindowRaw          = "raw"          // 原始：已入库，未分类
	WindowDecomposable = "decomposable" // 可分解：存在重叠脉冲，可执行解卷积
	WindowPiled        = "piled"        // 堆积：已识别为脉冲堆积，待分解
	WindowSaturated    = "saturated"    // 饱和：信号超出量程，标记为不可恢复
	WindowDuplicate    = "duplicate"    // 重复：触发序号冲突，跳过
)

// 脉冲事件状态机：
// candidate(候选) -> separated(已分离)/inseparable(不可分离)；separated 可被 confirmed(确认)。
const (
	PulseCandidate   = "candidate"   // 候选：峰值检测发现的可疑脉冲
	PulseSeparated   = "separated"   // 已分离：经解卷积从堆积中恢复出该脉冲
	PulseInseparable = "inseparable" // 不可分离：无法用参考脉冲解释，归入不可恢复区
	PulseConfirmed   = "confirmed"   // 确认：实验人员复核确认
)

// 计数快照状态机：
// draft(草稿) -> published(发布)；发布后可产生替代版本 superseded。
const (
	SnapshotDraft      = "draft"      // 草稿：正在汇总计数
	SnapshotPublished  = "published"  // 发布：已冻结为计数快照
	SnapshotSuperseded = "superseded" // 替代：被更新的计数版本取代
)

// Run 是一次辐射采集运行：承载探测器工况（采样率/死区时间）与波形窗口接收。
type Run struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	DetectorType  string    `json:"detector_type"`   // 探测器类型（NaI/HPGe/塑料闪烁体）
	SampleRateHz  float64   `json:"sample_rate_hz"`  // 采样率（Hz）
	DeadTimeNs    int64     `json:"dead_time_ns"`    // 死区时间（纳秒）
	Status        string    `json:"status"`
	Fingerprint   string    `json:"fingerprint"`     // 幂等指纹：名称+工况哈希
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WaveformWindow 是一段探测器波形窗口：携带波形抽样、触发序号与时间窗。
type WaveformWindow struct {
	ID            string    `json:"id"`
	RunID         string    `json:"run_id"`
	TriggerIndex  int64     `json:"trigger_index"`  // 触发序号（幂等键，单调递增）
	StartTimeNs   int64     `json:"start_time_ns"`  // 相对运行起始的纳秒偏移
	DurationNs    int64     `json:"duration_ns"`    // 窗口时长（纳秒）
	Samples       string    `json:"samples"`        // 波形抽样摘要（JSON 数组，归一化幅度）
	BaselineLevel float64   `json:"baseline_level"` // 基线水平（估计值）
	PeakAmplitude float64   `json:"peak_amplitude"` // 峰值幅度（归一化 0~1）
	Saturated     bool      `json:"saturated"`      // 是否饱和
	Status        string    `json:"status"`
	Fingerprint   string    `json:"fingerprint"`    // 幂等指纹：运行+触发序号哈希
	CreatedAt     time.Time `json:"created_at"`
}

// PulseEvent 是一个脉冲事件：从堆积波形中分离出的单个脉冲或不可分离残留。
type PulseEvent struct {
	ID            string    `json:"id"`
	RunID         string    `json:"run_id"`
	WindowID      string    `json:"window_id"`
	ArrivalTimeNs int64     `json:"arrival_time_ns"` // 到达时间偏移（纳秒）
	Amplitude     float64   `json:"amplitude"`       // 恢复幅度（归一化）
	GroupIndex    int       `json:"group_index"`     // 堆积组序号（孤立脉冲为 0）
	Status        string    `json:"status"`
	ResidualRatio float64   `json:"residual_ratio"` // 拟合后残差占比（0~1）
	Confidence    float64   `json:"confidence"`     // 置信度（0~1）
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// DeadZone 是一段不可恢复时间区：因饱和/基线漂移/不可分离堆积而无法计数。
type DeadZone struct {
	ID          string    `json:"id"`
	RunID       string    `json:"run_id"`
	StartTimeNs int64     `json:"start_time_ns"`
	EndTimeNs   int64     `json:"end_time_ns"`
	Reason      string    `json:"reason"` // saturated / baseline_drift / unresolvable_pileup
	CreatedAt   time.Time `json:"created_at"`
}

// BaselineRecord 记录一次运行的基线估计结果。
type BaselineRecord struct {
	ID           string    `json:"id"`
	RunID        string    `json:"run_id"`
	Level        float64   `json:"level"`         // 基线水平（归一化）
	DriftSlope   float64   `json:"drift_slope"`   // 漂移斜率（每窗口）
	NoiseFloor   float64   `json:"noise_floor"`   // 噪声底（均方根）
	WindowCount  int       `json:"window_count"`  // 参与估计的窗口数
	Locked       bool      `json:"locked"`        // 是否已锁定（锁定后不再更新）
	CreatedAt    time.Time `json:"created_at"`
}

// ReferencePulse 是锁定的参考脉冲形状：作为解卷积的匹配核。
type ReferencePulse struct {
	ID            string    `json:"id"`
	RunID         string    `json:"run_id"`
	Amplitude     float64   `json:"amplitude"`      // 归一化幅度（1.0）
	WidthNs       int64     `json:"width_ns"`       // 半高宽（纳秒）
	Shape         string    `json:"shape"`          // 归一化脉冲形状抽样（JSON 数组）
	SourceWindow  string    `json:"source_window"`  // 来源窗口 ID
	LockedAt      time.Time `json:"locked_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// CountSnapshot 是发布的计数快照：冻结计数与真实计数率的不可变快照。
type CountSnapshot struct {
	ID                      string     `json:"id"`
	RunID                   string     `json:"run_id"`
	Version                 int        `json:"version"`
	Status                  string     `json:"status"`
	TotalCounts             int        `json:"total_counts"`              // 总计数（含恢复）
	RecoveredCounts         int        `json:"recovered_counts"`          // 解卷积恢复的堆积计数
	UnresolvedCounts        int        `json:"unresolved_counts"`         // 不可分离计数
	ObservedCountRate       float64    `json:"observed_count_rate"`       // 观测计数率（counts/s）
	TrueCountRate           float64    `json:"true_count_rate"`           // 真实计数率（死区校正）
	EffectiveObservationNs  int64      `json:"effective_observation_ns"`  // 有效观察时间
	DeadTimeFraction        float64    `json:"dead_time_fraction"`        // 死区占比（0~1）
	UnrecoverableZones      int        `json:"unrecoverable_zones"`       // 不可恢复区数
	PulsesJSON              string     `json:"pulses_json"`               // 脉冲快照（JSON）
	Summary                 string     `json:"summary"`
	CreatedAt               time.Time  `json:"created_at"`
	PublishedAt             *time.Time `json:"published_at"`
}

// StatSummary 是自检/统计接口返回的全局快照。
type StatSummary struct {
	Runs       int `json:"runs"`
	Windows    int `json:"windows"`
	Pulses     int `json:"pulses"`
	Snapshots  int `json:"snapshots"`
	DeadZones  int `json:"dead_zones"`
	OpenRuns   int `json:"open_runs"`
}
