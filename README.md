# task223-pileup · 辐射探测器脉冲堆积解卷积服务

面向核探测实验人员的后端服务：从辐射探测器波形流中分离脉冲堆积事件，
估计真实计数率，并标记无法恢复的时间段（死区）。

## 业务能力

- 采集运行登记与生命周期（接收中 → 处理中 → 待复核 → 已完成 → 封存）；
- 波形窗口接收（触发序号幂等去重、饱和窗口自动识别）；
- 基线估计与漂移检测（滑动中位数 + 线性回归）；
- 参考脉冲锁定（解卷积的匹配核）；
- 脉冲堆积识别与受约束解卷积（贪心匹配追踪）；
- 死区标记（饱和 / 基线漂移 / 不可分离堆积）；
- 计数快照发布（死区校正后的真实计数率）；
- SQLite 持久化与重启恢复。

## 标准命令

```bash
# 构建 / 静态检查 / 测试 / 冒烟
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/pileup --smoke-test

# 启动服务
go run ./cmd/pileup --addr :8080 --db ./pileup.db
```

## 组件版本

- Go 1.26.3（GOTOOLCHAIN=local）
- SQLite 3.46.1（`modernc.org/sqlite` v1.52.0，纯 Go 驱动，CGO 无关）

## API 入口

统一 `/api` 前缀。主要路由：

| 能力 | 入口 |
| --- | --- |
| 运行 | `POST/GET /api/runs`, `GET /api/runs/{id}`, `POST /api/runs/{id}/finish`, `POST /api/runs/{id}/confirm` |
| 窗口 | `POST/GET /api/runs/{id}/windows`, `POST /api/windows/{id}/saturate` |
| 基线/参考 | `GET /api/runs/{id}/baseline`, `POST /api/runs/{id}/reference` |
| 解卷积 | `POST /api/runs/{id}/deconvolve` |
| 脉冲/死区 | `GET /api/runs/{id}/pulses`, `GET /api/pulses/{id}`, `POST /api/pulses/{id}/confirm|reject`, `GET /api/runs/{id}/deadzones` |
| 快照 | `POST/GET /api/runs/{id}/snapshots`, `GET /api/snapshots/{id}` |
| 统计/健康 | `GET /api/stats`, `GET /api/health` |

## 模块结构

```
internal/
├── model/       实体与状态机
├── store/       SQLite 持久化（7 表）
├── baseline/    基线估计与漂移检测
├── detector/    峰值检测与堆积识别
├── deconv/      受约束匹配追踪解卷积
├── deadzone/    死区标记与饱和检测
├── counting/    计数汇总与死区校正
├── snapshot/    计数快照构建
├── service/     编排层
└── httpapi/     HTTP 层
```
