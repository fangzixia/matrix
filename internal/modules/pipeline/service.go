package pipeline

import (
	"context"

	"github.com/google/uuid"

	"matrix/internal/platform/config"
)

type Service struct {
	cfg config.PipelineConfig
}

func NewService(cfg config.PipelineConfig) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) UpdateConfig(cfg config.PipelineConfig) {
	s.cfg = cfg
}

func (s *Service) DefaultStages() []string {
	if len(s.cfg.DefaultStages) > 0 {
		return append([]string(nil), s.cfg.DefaultStages...)
	}
	return []string{"spec", "implement", "verify", "build"}
}

func (s *Service) PullBeforeStage() bool {
	return s.cfg.PullBeforeStage
}

func (s *Service) ResolveStages(stages []string) []string {
	if len(stages) > 0 {
		return stages
	}
	return s.DefaultStages()
}

// StagesForRun returns ordered stage kinds for a pipeline run.
func (s *Service) StagesForRun(_ context.Context, _ uuid.UUID, requested []string) []string {
	return s.ResolveStages(requested)
}
