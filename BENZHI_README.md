基于 Go 实现的辐射探测器脉冲堆积解卷积服务，一款纯后端核探测分析服务，处理波形分离、堆积事件解卷积与可追溯结果发布。

# BENZHI 评测说明 · task223-pileup

辐射探测器脉冲堆积解卷积服务（纯后端，无前端）。

## 构建与运行

```bash
# 构建
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...

# 单元测试
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...

# 端到端冒烟（Docker CMD 的判据）
go run ./cmd/pileup --smoke-test
# 期望输出 "SMOKE TEST PASSED"，退出码 0

# 启动服务
go run ./cmd/pileup --addr :8080 --db ./pileup.db
```

## --smoke-test 契约

`--smoke-test` 不启动长驻服务，而是：

1. 创建采集运行（NaI，500 MHz 采样率，40 ns 死区）；
2. 接收 12 个波形窗口（孤立脉冲 / 堆积脉冲 / 饱和窗口）；
3. 验证触发序号幂等（重复序号被跳过）；
4. 结束接收 → 解卷积（恢复堆积脉冲、标记饱和死区）；
5. 确认运行 → 发布计数快照（封存）；
6. 验证封存后拒绝再接收窗口；
7. 关闭数据库，重开同一路径，验证运行封存状态、脉冲、快照均恢复。

全部通过则输出 `SMOKE TEST PASSED` 并以 0 退出码结束。

## Docker 双架构

```bash
bash build_benzhi_docker.sh <镜像名> linux/amd64
bash build_benzhi_docker.sh <镜像名> linux/arm64
docker run --rm <镜像名>:latest --smoke-test
```

镜像默认 `ENTRYPOINT ["/app/pileup"]` + `CMD ["--smoke-test"]`，
`docker run` 直接执行冒烟测试，不要追加位置参数。

## API 概览

统一 `/api` 前缀，21 个路由，涵盖运行生命周期、波形窗口、基线估计、
参考脉冲锁定、脉冲堆积解卷积、死区查询、计数快照发布与统计自检。

## 组件

- Go 1.26.3（GOTOOLCHAIN=local）
- SQLite 3.46.1（`modernc.org/sqlite` v1.52.0）
- 基础镜像：`docker.m.daocloud.io/library/golang:1.26.3-bookworm`
