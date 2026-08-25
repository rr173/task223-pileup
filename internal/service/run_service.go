package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"task223-pileup/internal/model"
	"task223-pileup/internal/store"
)

// RunService 采集运行生命周期服务。
type RunService struct {
	store      *store.RunStore
	pulseStore *store.PulseStore
}

// NewRunService 构造运行服务。
func NewRunService(s *store.RunStore, p *store.PulseStore) *RunService {
	return &RunService{store: s, pulseStore: p}
}

// Create 登记一次采集运行（初始状态 receiving）。
func (s *RunService) Create(name, description, detectorType string, sampleRateHz float64, deadTimeNs int64) (*model.Run, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: run name required", model.ErrInvalidInput)
	}
	if sampleRateHz <= 0 {
		return nil, fmt.Errorf("%w: sample rate must be positive", model.ErrInvalidInput)
	}
	if deadTimeNs <= 0 {
		return nil, fmt.Errorf("%w: dead time must be positive", model.ErrInvalidInput)
	}
	fp := runFingerprint(name, detectorType, sampleRateHz, deadTimeNs)
	now := time.Now().UTC()
	r := &model.Run{
		ID:           "run-" + shortHash(fp),
		Name:         name,
		Description:  description,
		DetectorType: detectorType,
		SampleRateHz: sampleRateHz,
		DeadTimeNs:   deadTimeNs,
		Status:       model.RunReceiving,
		Fingerprint:  fp,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.Create(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Get 读取运行。
func (s *RunService) Get(id string) (*model.Run, error) { return s.store.Get(id) }

// List 返回全部运行。
func (s *RunService) List() ([]model.Run, error) { return s.store.List() }

// FinishReceiving 接收中 -> 处理中（停止接收波形窗口）。
func (s *RunService) FinishReceiving(id string) (*model.Run, error) {
	return s.transition(id, model.RunReceiving, model.RunProcessing)
}

// CompleteProcessing 处理中 -> 待复核（解卷积完成，待实验人员复核）。
func (s *RunService) CompleteProcessing(id string) (*model.Run, error) {
	return s.transition(id, model.RunProcessing, model.RunPendingReview)
}

// Confirm 待复核 -> 已完成（复核通过）。
//
// 只有当所有待复核脉冲都已被实验人员确认或否决后，运行才能完成。
// 仍存在已分离但未复核脉冲时拒绝流转，避免发布不完整的计数快照。
func (s *RunService) Confirm(id string) (*model.Run, error) {
	pending, err := s.pulseStore.CountPendingReview(id)
	if err != nil {
		return nil, err
	}
	if pending > 0 {
		return nil, fmt.Errorf("%w: %d pulse(s) still pending review", model.ErrInvalidState, pending)
	}
	return s.transition(id, model.RunPendingReview, model.RunCompleted)
}

// Seal 已完成 -> 封存（仅由快照发布服务调用）。
func (s *RunService) Seal(id string) (*model.Run, error) {
	return s.transition(id, model.RunCompleted, model.RunSealed)
}

// transition 执行状态流转，非法流转返回 ErrInvalidState。
func (s *RunService) transition(id, expected, next string) (*model.Run, error) {
	ok, err := s.store.UpdateStatus(id, expected, next)
	if err != nil {
		return nil, err
	}
	if !ok {
		cur, err := s.store.Get(id)
		if err != nil {
			return nil, err
		}
		if cur.Status == model.RunSealed {
			return nil, model.ErrSealed
		}
		return nil, fmt.Errorf("%w: cannot move run from %s to %s", model.ErrInvalidState, cur.Status, next)
	}
	return s.store.Get(id)
}

func runFingerprint(name, detectorType string, sampleRateHz float64, deadTimeNs int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%.3f|%d", name, detectorType, sampleRateHz, deadTimeNs)))
	return hex.EncodeToString(h[:])
}

func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
