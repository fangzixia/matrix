package view

import (
	"context"
	"encoding/json"
	"matrix/internal/ai/agent"
	"matrix/internal/ai/stream"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/logging"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ViewPersistence 持久化 Run 视图快照。
type ViewPersistence interface {
	SaveView(ctx context.Context, row *models.RunView) error
	LoadView(ctx context.Context, runID uuid.UUID) (*models.RunView, error)
}

// Store 管理 Run 视图投影与 DB 持久化；SSE 只读 DB，不经内存推送。
type Store struct {
	persistence ViewPersistence
	mu          sync.Mutex
	runs        map[string]*runSession
}

type runSession struct {
	projector *Projector
	seq       int64
	projectID string
}

// NewStore 创建视图存储。
func NewStore(p ViewPersistence) *Store {
	return &Store{
		persistence: p,
		runs:        make(map[string]*runSession),
	}
}

// runViewBeginSeq 为 BeginRun 快照序号；seq=1 预留给 CatchUp 合成的 RUN_STARTED。
const runViewBeginSeq int64 = 2

// BeginRun 初始化 Run 视图会话并持久化初始快照。
func (s *Store) BeginRun(ctx context.Context, runID, projectID, kind, phase string) error {
	s.mu.Lock()
	proj := NewProjector(runID, projectID)
	proj.state.Status = "running"
	proj.state.StatusLabel = "Agent 正在工作…"
	proj.state.Phase = phase
	proj.state.Seq = runViewBeginSeq
	s.runs[runID] = &runSession{projector: proj, projectID: projectID, seq: runViewBeginSeq}
	s.mu.Unlock()
	logging.Agent("run-view: 视图已开始", "run_id", runID, "kind", kind)
	if err := s.persist(ctx, runID); err != nil {
		logging.Agent("run-view: 持久化失败", "run_id", runID, "phase", "begin", "error", err.Error())
		return err
	}
	return nil
}

// FinishRun 持久化终态视图；RUN_FINISHED 由 CatchUpAfterSeq 从 runs 表合成。
func (s *Store) FinishRun(ctx context.Context, runID, status, output, errMsg string) error {
	s.mu.Lock()
	sess := s.runs[runID]
	if sess != nil && sess.projector != nil {
		sess.projector.state.Status = status
		if status == "succeeded" {
			sess.projector.finalizeStaleTools(true)
		} else if status == "failed" || status == "cancelled" {
			sess.projector.finalizeStaleTools(false)
		}
		if output != "" {
			sess.projector.state.ReplyText = output
		}
		if errMsg != "" {
			sess.projector.state.Error = FormatUserRunError(errMsg)
		}
		sess.seq++
		sess.projector.state.Seq = sess.seq
	}
	persistRow := s.persistRowLocked(runID, sess)
	delete(s.runs, runID)
	s.mu.Unlock()

	logging.Agent("run-view: 视图已结束",
		"run_id", runID, "status", status, "output_len", len(output),
	)
	if persistRow.RunID == uuid.Nil && s.persistence != nil {
		st := NewRunViewState(runID)
		st.Status = status
		st.ReplyText = output
		st.Seq = 1
		if errMsg != "" {
			st.Error = FormatUserRunError(errMsg)
		}
		if rid, err := uuid.Parse(runID); err == nil {
			if b, err := json.Marshal(st); err == nil {
				persistRow = models.RunView{
					RunID: rid, Seq: st.Seq, State: string(b), UpdatedAt: time.Now(),
				}
			}
		}
	}
	if persistRow.RunID != uuid.Nil {
		if err := s.persistence.SaveView(ctx, &persistRow); err != nil {
			logging.Agent("run-view: 持久化失败", "run_id", runID, "phase", "finish", "error", err.Error())
			return err
		}
	}
	return nil
}

// SetStatusLabel 更新运行状态标签并持久化（如克隆仓库等准备阶段）。
func (s *Store) SetStatusLabel(ctx context.Context, runID, label string) {
	s.mu.Lock()
	sess := s.runs[runID]
	if sess != nil && sess.projector != nil {
		sess.projector.state.StatusLabel = label
		sess.seq++
		sess.projector.state.Seq = sess.seq
	}
	row := s.persistRowLocked(runID, sess)
	s.mu.Unlock()
	if row.RunID != uuid.Nil {
		_ = s.saveRow(ctx, row)
	}
}

// OnSubagent 更新子 Agent 快照并持久化。
func (s *Store) OnSubagent(ctx context.Context, runID string, snap agent.Snapshot) {
	s.mu.Lock()
	sess := s.runs[runID]
	if sess == nil || sess.projector == nil {
		s.mu.Unlock()
		return
	}
	sess.projector.OnSubagent(snap)
	sess.seq++
	sess.projector.state.Seq = sess.seq
	row := s.persistRowLocked(runID, sess)
	s.mu.Unlock()
	_ = s.saveRow(ctx, row)
}

// Snapshot 返回当前视图状态；无内存会话时从 DB 加载。
func (s *Store) Snapshot(ctx context.Context, runID string) (*RunViewState, error) {
	s.mu.Lock()
	sess := s.runs[runID]
	if sess != nil && sess.projector != nil {
		st := sess.projector.State()
		st.Seq = sess.seq
		s.mu.Unlock()
		return &st, nil
	}
	s.mu.Unlock()
	if s.persistence == nil {
		return nil, nil
	}
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	row, err := s.persistence.LoadView(ctx, rid)
	if err != nil || row == nil {
		return nil, err
	}
	var st RunViewState
	if err := json.Unmarshal([]byte(row.State), &st); err != nil {
		return nil, err
	}
	st.Seq = row.Seq
	return &st, nil
}

// Sink 返回将 stream.Message 投影并持久化的 Sink。
func (s *Store) Sink(runID, projectID string) stream.Sink {
	return stream.FuncSink(func(ctx context.Context, msg stream.Message) error {
		return s.apply(ctx, runID, projectID, msg)
	})
}

func (s *Store) apply(ctx context.Context, runID, projectID string, msg stream.Message) error {
	s.mu.Lock()
	sess := s.runs[runID]
	if sess == nil {
		sess = &runSession{projector: NewProjector(runID, projectID), projectID: projectID}
		s.runs[runID] = sess
	}
	sess.projector.Apply(msg)
	sess.seq++
	sess.projector.state.Seq = sess.seq
	row := s.persistRowLocked(runID, sess)
	s.mu.Unlock()
	return s.saveRow(ctx, row)
}

func (s *Store) persistRowLocked(runID string, sess *runSession) models.RunView {
	if sess == nil || sess.projector == nil || s.persistence == nil {
		return models.RunView{}
	}
	st := sess.projector.State()
	st.Seq = sess.seq
	b, err := json.Marshal(st)
	if err != nil {
		return models.RunView{}
	}
	rid, err := uuid.Parse(runID)
	if err != nil {
		return models.RunView{}
	}
	return models.RunView{
		RunID: rid, Seq: sess.seq, State: string(b), UpdatedAt: time.Now(),
	}
}

func (s *Store) persist(ctx context.Context, runID string) error {
	s.mu.Lock()
	row := s.persistRowLocked(runID, s.runs[runID])
	s.mu.Unlock()
	return s.saveRow(ctx, row)
}

func (s *Store) saveRow(ctx context.Context, row models.RunView) error {
	if row.RunID == uuid.Nil {
		return nil
	}
	return s.persistence.SaveView(ctx, &row)
}
