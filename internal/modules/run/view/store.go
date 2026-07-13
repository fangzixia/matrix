package view

import (
	"context"
	"encoding/json"
	ai "matrix/ai/sdk"
	"matrix/internal/platform/db/models"
	"matrix/internal/platform/logging"
	"sync"
	"time"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/google/uuid"
)

// ViewPersistence 持久化 Run 视图快照与事件日志。
type ViewPersistence interface {
	SaveView(ctx context.Context, row *models.RunView) error
	LoadView(ctx context.Context, runID uuid.UUID) (*models.RunView, error)
	EventLogPersistence
}

// Store 管理 Run 视图投影、事件日志与 DB 持久化；SSE 只读 DB。
type Store struct {
	persistence ViewPersistence
	eventLog    *EventLog
	mu          sync.Mutex
	runs        map[string]*runSession
}

type runSession struct {
	projector *Projector
	seq       int64
	projectID string
	lastSnap  time.Time
}

// NewStore 创建视图存储。
func NewStore(p ViewPersistence) *Store {
	return &Store{
		persistence: p,
		eventLog:    NewEventLog(p),
		runs:        make(map[string]*runSession),
	}
}

// runViewBeginSeq 为 BeginRun 快照序号；seq=1 预留给 CatchUp 合成的 RUN_STARTED。
const runViewBeginSeq int64 = 2

const stateSnapshotThrottle = 500 * time.Millisecond

// BeginRun 初始化 Run 视图会话并持久化初始快照。
func (s *Store) BeginRun(ctx context.Context, jobID, projectID, kind, phase string) error {
	s.mu.Lock()
	proj := NewProjector(jobID, projectID)
	proj.state.Status = "running"
	proj.state.StatusLabel = "Agent 正在工作…"
	proj.state.Phase = phase
	proj.state.Seq = runViewBeginSeq
	s.runs[jobID] = &runSession{projector: proj, projectID: projectID, seq: runViewBeginSeq}
	s.mu.Unlock()
	logging.Agent("run-view: 视图已开始", "run_id", jobID, "kind", kind)
	return s.persist(ctx, jobID)
}

// FinishRun 持久化终态视图；Job 终态 RUN_FINISHED 由 CatchUpAfterSeq 合成。
func (s *Store) FinishRun(ctx context.Context, jobID, status, output, errMsg string) error {
	s.mu.Lock()
	sess := s.runs[jobID]
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
	persistRow := s.persistRowLocked(jobID, sess)
	delete(s.runs, jobID)
	s.mu.Unlock()

	logging.Agent("run-view: 视图已结束",
		"run_id", jobID, "status", status, "output_len", len(output),
	)
	if persistRow.RunID == uuid.Nil && s.persistence != nil {
		st := NewRunViewState(jobID)
		st.Status = status
		st.ReplyText = output
		st.Seq = 1
		if errMsg != "" {
			st.Error = FormatUserRunError(errMsg)
		}
		if rid, err := uuid.Parse(jobID); err == nil {
			if b, err := json.Marshal(st); err == nil {
				persistRow = models.RunView{
					RunID: rid, Seq: st.Seq, State: string(b), UpdatedAt: time.Now(),
				}
			}
		}
	}
	if persistRow.RunID != uuid.Nil {
		if err := s.persistence.SaveView(ctx, &persistRow); err != nil {
			logging.Agent("run-view: 持久化失败", "run_id", jobID, "phase", "finish", "error", err.Error())
			return err
		}
	}
	return nil
}

// SetStatusLabel 更新运行状态标签并持久化（如克隆仓库等准备阶段）。
func (s *Store) SetStatusLabel(ctx context.Context, jobID, label string) {
	s.mu.Lock()
	sess := s.runs[jobID]
	if sess != nil && sess.projector != nil {
		sess.projector.state.StatusLabel = label
		sess.seq++
		sess.projector.state.Seq = sess.seq
	}
	row := s.persistRowLocked(jobID, sess)
	s.mu.Unlock()
	if row.RunID != uuid.Nil {
		_ = s.saveRow(ctx, row)
	}
}

// OnSubagent 更新子 Agent 快照并持久化。
func (s *Store) OnSubagent(ctx context.Context, jobID string, snap ai.AgentSnapshot) {
	s.mu.Lock()
	sess := s.runs[jobID]
	if sess == nil || sess.projector == nil {
		s.mu.Unlock()
		return
	}
	sess.projector.OnSubagent(snap)
	sess.seq++
	sess.projector.state.Seq = sess.seq
	row := s.persistRowLocked(jobID, sess)
	s.mu.Unlock()
	_ = s.saveRow(ctx, row)
}

// Snapshot 返回当前视图状态；无内存会话时从 DB 加载。
func (s *Store) Snapshot(ctx context.Context, jobID string) (*RunViewState, error) {
	s.mu.Lock()
	sess := s.runs[jobID]
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
	rid, err := uuid.Parse(jobID)
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

// Sink 返回将 AG-UI 事件投影、持久化事件日志与快照的 Sink。
func (s *Store) Sink(jobID, projectID string) ai.Sink {
	return ai.FuncSink(func(ctx context.Context, ev ai.Event) error {
		return s.applyEvent(ctx, jobID, projectID, ev)
	})
}

func (s *Store) applyEvent(ctx context.Context, jobID, projectID string, ev ai.Event) error {
	if ev == nil {
		return nil
	}
	s.mu.Lock()
	sess := s.runs[jobID]
	if sess == nil {
		sess = &runSession{projector: NewProjector(jobID, projectID), projectID: projectID}
		s.runs[jobID] = sess
	}
	sess.seq++
	seq := sess.seq
	if ev.Type() != agui.EventTypeStateSnapshot {
		sess.projector.ApplyEvent(ev)
		sess.projector.state.Seq = seq
	}
	row := s.persistRowLocked(jobID, sess)
	shouldSnap := ev.Type() != agui.EventTypeStateSnapshot && time.Since(sess.lastSnap) >= stateSnapshotThrottle
	var snapEv ai.Event
	if shouldSnap {
		st := sess.projector.State()
		st.Seq = seq
		snapEv = StateSnapshot(st)
		sess.lastSnap = time.Now()
	}
	s.mu.Unlock()

	eventJSON, err := ev.ToJSON()
	if err != nil {
		return err
	}
	jid, err := uuid.Parse(jobID)
	if err != nil {
		return err
	}
	if err := s.eventLog.Append(ctx, jid, seq, string(ev.Type()), string(eventJSON)); err != nil {
		logging.Agent("run-view: 事件日志写入失败", "run_id", jobID, "seq", seq, "error", err.Error())
		return err
	}
	if err := s.saveRow(ctx, row); err != nil {
		return err
	}
	if snapEv != nil {
		return s.applyEvent(ctx, jobID, projectID, snapEv)
	}
	return nil
}

func (s *Store) persistRowLocked(jobID string, sess *runSession) models.RunView {
	if sess == nil || sess.projector == nil || s.persistence == nil {
		return models.RunView{}
	}
	st := sess.projector.State()
	st.Seq = sess.seq
	b, err := json.Marshal(st)
	if err != nil {
		return models.RunView{}
	}
	rid, err := uuid.Parse(jobID)
	if err != nil {
		return models.RunView{}
	}
	return models.RunView{
		RunID: rid, Seq: sess.seq, State: string(b), UpdatedAt: time.Now(),
	}
}

func (s *Store) persist(ctx context.Context, jobID string) error {
	s.mu.Lock()
	row := s.persistRowLocked(jobID, s.runs[jobID])
	s.mu.Unlock()
	return s.saveRow(ctx, row)
}

func (s *Store) saveRow(ctx context.Context, row models.RunView) error {
	if row.RunID == uuid.Nil {
		return nil
	}
	return s.persistence.SaveView(ctx, &row)
}

// EventLog 返回事件日志访问器（CatchUp 使用）。
func (s *Store) EventLog() *EventLog {
	return s.eventLog
}
