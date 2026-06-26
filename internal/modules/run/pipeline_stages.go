package run

import (
	"context"
	"errors"

	"matrix/internal/ai/ports"
	"matrix/internal/platform/db/models"
)

// pipelineStageKinds 从 run_steps 读取流水线阶段。
func (s *Service) pipelineStageKinds(ctx context.Context, m *models.Run) ([]string, error) {
	var rows []models.RunStep
	if err := s.db.WithContext(ctx).Where("run_id = ?", m.ID).Order("sequence asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		kinds := make([]string, len(rows))
		for i := range rows {
			kinds[i] = rows[i].Kind
		}
		return kinds, nil
	}
	if s.pipeline == nil {
		return nil, errors.New("流水线未配置")
	}
	return s.pipeline.DefaultStages(), nil
}

func runOutputFromResult(result ports.RunResult) string {
	output := result.Output
	if output == "" && len(result.Messages) > 0 {
		output = result.Messages[len(result.Messages)-1].Content
	}
	return output
}
