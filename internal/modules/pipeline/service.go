// Package pipeline Harness 流水线阶段配置（Plan/Implement/Verify/Build 等）。
package pipeline

import (
	"context"
	"matrix/internal/platform/config"

	"github.com/google/uuid"
)

// Service 管理 Harness 流水线阶段配置（Spec/Implement/Verify/Build 等）。
type Service struct {
	cfg config.PipelineConfig
}

// NewService 创建流水线服务实例。
func NewService(cfg config.PipelineConfig) *Service {
	return &Service{cfg: cfg}
}

// UpdateConfig 更新流水线阶段配置。
func (s *Service) UpdateConfig(cfg config.PipelineConfig) {
	s.cfg = cfg
}

// DefaultStages 返回默认流水线阶段列表。
func (s *Service) DefaultStages() []string {
	if len(s.cfg.DefaultStages) > 0 {
		return append([]string(nil), s.cfg.DefaultStages...)
	}
	return []string{"plan", "build"}
}

// PullBeforeStage 返回阶段执行前是否拉取 Git。
func (s *Service) PullBeforeStage() bool {
	return s.cfg.PullBeforeStage
}

// ResolveStages 解析并返回 Run 的流水线阶段。
func (s *Service) ResolveStages(stages []string) []string {
	if len(stages) > 0 {
		return stages
	}
	return s.DefaultStages()
}

// StagesForRun 返回流水线 Run 的有序阶段种类列表。
func (s *Service) StagesForRun(_ context.Context, _ uuid.UUID, requested []string) []string {
	return s.ResolveStages(requested)
}
