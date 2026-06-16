package run

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"

	"matrix/internal/ai/ports"
	"matrix/internal/ai/stream"
	"matrix/internal/modules/project"
	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/storage"
)

type StepDTO struct {
	ID            uuid.UUID  `json:"id"`
	RunID         uuid.UUID  `json:"run_id"`
	Kind          string     `json:"kind"`
	Sequence      int        `json:"sequence"`
	Status        string     `json:"status"`
	OutputSummary string     `json:"output_summary,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type EventDTO struct {
	ID        uuid.UUID  `json:"id"`
	RunID     uuid.UUID  `json:"run_id"`
	StepID    *uuid.UUID `json:"step_id,omitempty"`
	EventType string     `json:"event_type"`
	Payload   string     `json:"payload,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Service) ListSteps(ctx context.Context, runID uuid.UUID) ([]StepDTO, error) {
	var rows []models.RunStep
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).Order("sequence asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]StepDTO, len(rows))
	for i := range rows {
		out[i] = StepDTO{
			ID: rows[i].ID, RunID: rows[i].RunID, Kind: rows[i].Kind, Sequence: rows[i].Sequence,
			Status: rows[i].Status, OutputSummary: rows[i].OutputSummary,
			StartedAt: rows[i].StartedAt, FinishedAt: rows[i].FinishedAt,
		}
	}
	return out, nil
}

func (s *Service) ListEvents(ctx context.Context, runID uuid.UUID, afterID *uuid.UUID, limit int) ([]EventDTO, error) {
	if limit <= 0 {
		limit = 200
	}
	q := s.db.WithContext(ctx).Where("run_id = ?", runID)
	if afterID != nil && *afterID != uuid.Nil {
		q = q.Where("id > ?", *afterID)
	}
	var rows []models.RunEvent
	if err := q.Order("created_at asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]EventDTO, len(rows))
	for i := range rows {
		out[i] = EventDTO{
			ID: rows[i].ID, RunID: rows[i].RunID, StepID: rows[i].StepID,
			EventType: rows[i].EventType, Payload: rows[i].Payload, CreatedAt: rows[i].CreatedAt,
		}
	}
	return out, nil
}

func (s *Service) GetAudit(ctx context.Context, runID uuid.UUID) (string, error) {
	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return "", err
	}
	if m.AuditPath == "" {
		return "", nil
	}
	b, err := os.ReadFile(m.AuditPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Service) persistEvent(ctx context.Context, runID uuid.UUID, stepID *uuid.UUID, msg stream.Message) {
	payload, _ := json.Marshal(msg)
	ev := models.RunEvent{
		RunID: runID, StepID: stepID, EventType: msg.Type, Payload: string(payload),
	}
	_ = s.db.WithContext(ctx).Create(&ev).Error
}

func (s *Service) compositeSink(runID uuid.UUID, stepID *uuid.UUID) stream.Sink {
	base := s.hub.Sink(runID.String())
	return stream.FuncSink(func(ctx context.Context, msg stream.Message) error {
		s.persistEvent(ctx, runID, stepID, msg)
		return base.Publish(ctx, msg)
	})
}

func (s *Service) ExecuteRun(ctx context.Context, runID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var m models.Run
	if err := s.db.WithContext(ctx).First(&m, "id = ?", runID).Error; err != nil {
		return err
	}

	switch m.Status {
	case "succeeded", "cancelled":
		return nil
	}

	unlock := s.sandboxLocks.acquire(m.ProjectID, m.RepositoryID)
	defer unlock()

	now := time.Now()
	_ = s.db.WithContext(ctx).Model(&m).Updates(map[string]any{
		"status": "running", "started_at": now, "finished_at": nil, "error_message": "",
	}).Error
	if s.notifier != nil {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, "running", m.Title)
	}

	var runErr error
	if m.Kind == "pipeline" {
		runErr = s.executePipeline(ctx, &m)
	} else {
		runErr = s.executeSingle(ctx, &m, m.Kind, nil)
	}

	if ctx.Err() != nil {
		runErr = ctx.Err()
	}

	fin := time.Now()
	status := "succeeded"
	errMsg := ""
	if runErr != nil {
		status = "failed"
		errMsg = runErr.Error()
	}
	_ = s.db.WithContext(ctx).Model(&models.Run{}).Where("id = ?", runID).Updates(map[string]any{
		"status": status, "finished_at": fin, "error_message": errMsg,
	}).Error
	if s.notifier != nil {
		s.notifier.NotifyRunStatus(ctx, m.CreatedBy, m.ProjectID, runID, status, m.Title)
	}
	s.hub.Publish(runID.String(), stream.ResultSuccessMsg(runID.String(), "", "", 0, time.Since(now)))
	return runErr
}

func (s *Service) loadExistingSteps(ctx context.Context, runID uuid.UUID) (map[int]*models.RunStep, error) {
	var rows []models.RunStep
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).Order("sequence asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	bySeq := make(map[int]*models.RunStep, len(rows))
	for i := range rows {
		bySeq[rows[i].Sequence] = &rows[i]
	}
	return bySeq, nil
}

func (s *Service) executePipeline(ctx context.Context, m *models.Run) error {
	if s.pipeline == nil {
		return errors.New("pipeline not configured")
	}
	stages := s.pipeline.StagesForRun(ctx, m.ID, decodePipelineStages(m.PipelineStages))
	existing, err := s.loadExistingSteps(ctx, m.ID)
	if err != nil {
		return err
	}

	for i, kind := range stages {
		if err := ctx.Err(); err != nil {
			return err
		}

		seq := i + 1
		if prev, ok := existing[seq]; ok && prev.Status == "succeeded" {
			continue
		}

		var stepID uuid.UUID
		start := time.Now()
		if prev, ok := existing[seq]; ok {
			stepID = prev.ID
			_ = s.db.WithContext(ctx).Model(prev).Updates(map[string]any{
				"status": "running", "started_at": start, "finished_at": nil, "output_summary": "",
			}).Error
		} else {
			step := models.RunStep{
				RunID: m.ID, Kind: kind, Sequence: seq, Status: "running", StartedAt: &start,
			}
			if err := s.db.WithContext(ctx).Create(&step).Error; err != nil {
				return err
			}
			stepID = step.ID
			existing[seq] = &step
		}

		if s.pipeline.PullBeforeStage() && s.pullAll != nil {
			_ = s.pullAll(ctx, m.ProjectID)
		}
		err := s.executeSingle(ctx, m, kind, &stepID)
		fin := time.Now()
		stepStatus := "succeeded"
		summary := ""
		if err != nil {
			stepStatus = "failed"
			summary = err.Error()
		}
		_ = s.db.WithContext(ctx).Model(&models.RunStep{}).Where("id = ?", stepID).Updates(map[string]any{
			"status": stepStatus, "finished_at": fin, "output_summary": summary,
		}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) executeSingle(ctx context.Context, m *models.Run, kind string, stepID *uuid.UUID) error {
	modelCfg := s.cfg.AI.ActiveModel()
	var mcpMerged map[string]config.MCPServerYAML
	if s.settings != nil {
		if integ, err := s.settings.GetIntegrations(ctx, m.ProjectID); err == nil && integ != nil {
			modelCfg = project.MergeModel(modelCfg, integ.Model)
			mcpMerged = project.MergeMCP(s.cfg.MCP.Servers, integ.MCPServers)
		}
	}
	msg := m.Title
	repoRoot, err := s.workspace.RepoRootFor(ctx, m.ProjectID, m.RepositoryID)
	if err != nil {
		return err
	}
	messages := BuildHarnessMessages(kind, msg, "", repoRoot)
	sessionsDir := storageProjectSessions(s.paths, m.ProjectID.String())
	sink := s.compositeSink(m.ID, stepID)
	mcpPorts := mcpConfigsToPorts(mcpMerged)
	if mcpPorts == nil {
		mcpPorts = mcpConfigsToPorts(s.cfg.MCP.Servers)
	}
	result, runErr := s.runtime.Run(ctx, ports.RunRequest{
		RunID: m.ID.String(), Kind: kind, Messages: messages,
		SandboxDir: repoRoot, SessionsDir: sessionsDir,
		Model: ports.ModelConfig{
			BaseURL: modelCfg.BaseURL, APIKey: modelCfg.APIKey,
			Model: modelCfg.Model, MaxTokens: modelCfg.MaxTokens,
		},
		MCP: mcpPorts,
		Policy: ports.RuntimePolicy{
			AllowShell: s.cfg.AI.Security.AllowShell, AllowCommandMCP: s.cfg.AI.Security.AllowCommandMCP,
		},
	}, sink)
	if runErr != nil {
		return runErr
	}
	if result.Err != nil {
		return result.Err
	}
	return nil
}

func storageProjectSessions(paths storage.Paths, projectID string) string {
	return storage.ProjectSessionsDir(paths, projectID)
}
