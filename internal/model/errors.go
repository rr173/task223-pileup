// Package model 定义辐射探测器脉冲堆积解卷积服务的领域实体、
// 状态常量与领域错误。实体为纯数据载体，不含持久化与 HTTP 逻辑，
// 业务规则由 baseline/detector/deconv/deadzone/counting/snapshot 各业务包承载。
package model

import "errors"

// 领域错误哨兵：service 与 httpapi 依据它们做错误映射。
var (
	// ErrNotFound 目标资源不存在。
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput 入参非法（采样率不一致、触发序号倒退、波形长度错误等）。
	ErrInvalidInput = errors.New("invalid input")
	// ErrInvalidState 状态机不允许的流转（如对已封存运行继续写入）。
	ErrInvalidState = errors.New("invalid state transition")
	// ErrConflict 并发冲突或重复写入。
	ErrConflict = errors.New("conflict")
	// ErrSealed 目标已封存，不可修改。
	ErrSealed = errors.New("sealed")
	// ErrDuplicate 幂等指纹冲突（重复触发序号）。
	ErrDuplicate = errors.New("duplicate")
	// ErrInsufficientData 数据不足（无法估计基线或解卷积）。
	ErrInsufficientData = errors.New("insufficient data")
	// ErrDeconvFailed 解卷积失败（无参考脉冲、残差不收敛等）。
	ErrDeconvFailed = errors.New("deconvolution failed")
)
