package view

import (
	"encoding/json"
	"fmt"
	"strings"

	ai "matrix/ai/sdk"
	"matrix/internal/modules/run/view/activity"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

type eventScope struct {
	worker     bool
	agentID    string
	parentTool string
	messageID  string
}

// ApplyEvent 处理一条 AG-UI 事件并更新 RunViewState。
func (p *Projector) ApplyEvent(ev ai.Event) {
	if ev == nil {
		return
	}
	scope := eventScopeFrom(ev)
	switch ev.Type() {
	case agui.EventTypeStepStarted:
		if step, ok := ev.(*agui.StepStartedEvent); ok {
			turn := parseTurnStep(step.StepName)
			if scope.worker {
				p.ensureWorkerTurnFromScope(fakeWorkerMsg(scope), turn, "")
			} else {
				p.ensureCoordinatorTurn(turn, "")
			}
			p.state.StatusLabel = activity.TurnThinkingLabel(turn, "")
		}
	case agui.EventTypeTextMessageStart:
		if msg, ok := ev.(*agui.TextMessageStartEvent); ok {
			scope.messageID = msg.MessageID
			p.messageID = msg.MessageID
			turn := p.activeTurnScope(scope)
			if turn != nil {
				turn.MessageStreaming = true
			}
		}
	case agui.EventTypeReasoningMessageStart:
		if msg, ok := ev.(*agui.ReasoningMessageStartEvent); ok {
			scope.messageID = msg.MessageID
			if p.messageID == "" {
				p.messageID = msg.MessageID
			}
			turn := p.activeTurnScope(scope)
			if turn != nil {
				turn.ThinkingStreaming = true
			}
		}
	case agui.EventTypeTextMessageContent:
		if msg, ok := ev.(*agui.TextMessageContentEvent); ok {
			turn := p.activeTurnScope(scope)
			if turn != nil {
				turn.Message += msg.Delta
				p.refreshTurnSummary(turn)
			}
		}
	case agui.EventTypeReasoningMessageContent:
		if msg, ok := ev.(*agui.ReasoningMessageContentEvent); ok {
			turn := p.activeTurnScope(scope)
			if turn != nil {
				turn.Thinking += msg.Delta
				p.refreshTurnSummary(turn)
			}
		}
	case agui.EventTypeTextMessageEnd, agui.EventTypeReasoningMessageEnd:
		turn := p.activeTurnScope(scope)
		if turn != nil {
			turn.MessageStreaming = false
			turn.ThinkingStreaming = false
		}
	case agui.EventTypeToolCallStart:
		if tc, ok := ev.(*agui.ToolCallStartEvent); ok {
			turn := p.resolveToolTurnScope(scope, tc.ToolCallID)
			if turn == nil {
				turn = p.activeTurnScope(scope)
			}
			if turn != nil {
				p.upsertToolInTurn(turn, tc.ToolCallID, tc.ToolCallName, "loading", "", "", 0)
				p.refreshTurnSummary(turn)
				p.rememberCoordinatorToolTurn(scope, tc.ToolCallID)
				p.state.StatusLabel = activity.ToolActivityLabel(tc.ToolCallName, "started")
			}
		}
	case agui.EventTypeToolCallArgs:
		if tc, ok := ev.(*agui.ToolCallArgsEvent); ok {
			turn := p.resolveToolTurnScope(scope, tc.ToolCallID)
			if turn == nil {
				turn = p.activeTurnScope(scope)
			}
			if turn == nil {
				return
			}
			tool := p.upsertToolInTurn(turn, tc.ToolCallID, "", "loading", "", "", 0)
			if tool != nil {
				tool.Preview = tool.Preview + tc.Delta
			}
			p.refreshTurnSummary(turn)
		}
	case agui.EventTypeCustom:
		if ai.IsToolOutputDelta(ev) {
			toolCallID, toolName, delta, ok := ai.ToolOutputDeltaFields(ev)
			if !ok {
				return
			}
			turn := p.resolveToolTurnScope(scope, toolCallID)
			if turn == nil {
				turn = p.activeTurnScope(scope)
			}
			if turn != nil {
				p.appendToolOutputDelta(turn, toolCallID, toolName, delta)
				p.refreshTurnSummary(turn)
			}
			if toolName != "" {
				p.state.StatusLabel = activity.ToolActivityLabel(toolName, "streaming")
			}
		}
	case agui.EventTypeToolCallEnd:
		if tc, ok := ev.(*agui.ToolCallEndEvent); ok {
			turn := p.resolveToolTurnScope(scope, tc.ToolCallID)
			if turn != nil {
				if tool := p.findToolInTurn(turn, tc.ToolCallID); tool != nil {
					tool.OutputStreaming = false
				}
			}
		}
	case agui.EventTypeToolCallResult:
		if tr, ok := ev.(*agui.ToolCallResultEvent); ok {
			turn := p.resolveToolTurnScope(scope, tr.ToolCallID)
			if turn == nil {
				turn = p.activeTurnScope(scope)
			}
			if turn == nil {
				return
			}
			status := "success"
			if strings.Contains(strings.ToLower(tr.Content), "error") || strings.Contains(tr.Content, "失败") {
				status = "error"
			}
			tool := p.upsertToolInTurn(turn, tr.ToolCallID, "", status, "", tr.Content, 0)
			if tool != nil {
				tool.OutputStreaming = false
			}
			p.refreshTurnSummary(turn)
			p.lastSnap = p.lastSnap // force snapshot on tool done via store throttle
			p.state.StatusLabel = activity.TurnWithToolsLabel(turn.Turn, countFinishedTools(turn.Tools))
		}
	case agui.EventTypeActivitySnapshot:
		if act, ok := ev.(*agui.ActivitySnapshotEvent); ok && act.ActivityType == ai.ActivityTypeSubagent {
			p.applySubagentSnapshot(act.Content)
		}
	case agui.EventTypeRunFinished:
		if rf, ok := ev.(*agui.RunFinishedEvent); ok {
			p.applyRunFinishedResult(rf.Result)
		}
	case agui.EventTypeRunError:
		if re, ok := ev.(*agui.RunErrorEvent); ok {
			p.state.Error = FormatUserRunError(re.Message)
		}
	case agui.EventTypeStateSnapshot:
		// 宿主兜底快照：由 store 直接处理，投影器忽略。
	}

	p.syncReplyText()
}

func (p *Projector) applyRunFinishedResult(result any) {
	if result == nil {
		return
	}
	m, ok := result.(map[string]any)
	if !ok {
		b, err := json.Marshal(result)
		if err != nil {
			return
		}
		_ = json.Unmarshal(b, &m)
	}
	if m == nil {
		return
	}
	output, _ := m["output"].(string)
	if output != "" {
		p.state.ReplyText = strings.TrimSpace(output)
		p.state.Result = &ResultView{Output: output, Subtype: "success"}
	}
	if errMsg, _ := m["error"].(string); errMsg != "" {
		p.state.Error = FormatUserRunError(errMsg)
	}
}

func eventScopeFrom(ev ai.Event) eventScope {
	s := eventScope{}
	switch e := ev.(type) {
	case *agui.TextMessageStartEvent:
		if e.Name != "" {
			s.worker = true
			s.agentID = e.Name
		}
	case *agui.TextMessageChunkEvent:
		if e.Name != nil && *e.Name != "" {
			s.worker = true
			s.agentID = *e.Name
		}
	case *agui.ActivitySnapshotEvent:
		if e.ActivityType == ai.ActivityTypeSubagent {
			if snap, ok := e.Content.(map[string]any); ok {
				if id, _ := snap["id"].(string); id != "" {
					s.worker = true
					s.agentID = id
				}
				if pt, _ := snap["parent_tool_use_id"].(string); pt != "" {
					s.parentTool = pt
				}
			}
		}
	}
	return s
}

func (p *Projector) activeTurnScope(scope eventScope) *TurnView {
	if scope.worker {
		wk := scope.agentID
		if scope.parentTool != "" {
			wk = scope.parentTool + ":" + scope.agentID
		} else if scope.agentID != "" {
			if pt := p.parse.workerParentToolID[scope.agentID]; pt != "" {
				wk = pt + ":" + scope.agentID
			}
		}
		if wk != "" {
			if t, ok := p.parse.workerCurrentTurn[wk]; ok {
				return t
			}
		}
		msg := fakeWorkerMsg(scope)
		return p.ensureWorkerTurnFromScope(msg, 1, "")
	}
	if p.parse.coordinatorTurn == nil {
		return p.ensureCoordinatorTurn(1, "")
	}
	return p.parse.coordinatorTurn
}

func fakeWorkerMsg(scope eventScope) workerMsg {
	return workerMsg{
		scope:      scope.worker,
		agentID:    scope.agentID,
		parentTool: scope.parentTool,
	}
}

type workerMsg struct {
	scope      bool
	agentID    string
	parentTool string
}

func (m workerMsg) Scope() string {
	if m.scope {
		return "worker"
	}
	return "coordinator"
}

func (p *Projector) ensureWorkerTurnFromScope(msg workerMsg, turnNum int, summary string) *TurnView {
	parentID := msg.parentTool
	if parentID == "" && msg.agentID != "" {
		parentID = p.parse.workerParentToolID[msg.agentID]
	}
	tool := p.findTool(parentID)
	wk := ""
	if parentID != "" && msg.agentID != "" {
		wk = parentID + ":" + msg.agentID
	}
	if tool == nil && parentID != "" {
		tool = p.ensureParentAgentToolPlaceholder(parentID)
	}
	if tool == nil {
		if wk != "" {
			if existing, ok := p.parse.workerCurrentTurn[wk]; ok {
				if summary != "" {
					existing.Summary = summary
				}
				return existing
			}
		}
		return nil
	}
	if wk == "" {
		return nil
	}
	if existing, ok := p.parse.workerCurrentTurn[wk]; ok && existing.Turn == turnNum {
		if summary != "" {
			existing.Summary = summary
		}
		return existing
	}
	turn := TurnView{
		Key:             fmt.Sprintf("worker-%s-turn-%d", msg.agentID, turnNum),
		Turn:            turnNum,
		Scope:           "worker",
		AgentID:         msg.agentID,
		ParentToolUseID: parentID,
		Summary:         summary,
		Tools:           []ToolView{},
	}
	tool.WorkerTurns = append(tool.WorkerTurns, turn)
	idx := len(tool.WorkerTurns) - 1
	p.parse.workerCurrentTurn[wk] = &tool.WorkerTurns[idx]
	return p.parse.workerCurrentTurn[wk]
}

func (p *Projector) resolveToolTurnScope(scope eventScope, toolUseID string) *TurnView {
	if toolUseID != "" {
		if _, turn := p.findToolGlobal(toolUseID); turn != nil {
			return turn
		}
	}
	return p.activeTurnScope(scope)
}

func (p *Projector) rememberCoordinatorToolTurn(scope eventScope, toolUseID string) {
	if scope.worker || toolUseID == "" || p.parse.coordinatorTurn == nil {
		return
	}
	p.parse.parentToolCoordinatorTurn[toolUseID] = p.parse.coordinatorTurn.Turn
}

func (p *Projector) applySubagentSnapshot(snapshot any) {
	if snapshot == nil {
		return
	}
	switch v := snapshot.(type) {
	case ai.AgentSnapshot:
		p.setSubagentFromSnapshot(v)
	case map[string]any:
		p.setSubagentFromMap(v)
	default:
		b, err := json.Marshal(snapshot)
		if err != nil {
			return
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			p.setSubagentFromMap(m)
		}
	}
}

func parseTurnStep(stepName string) int {
	stepName = strings.TrimPrefix(stepName, "turn-")
	n := 0
	for _, c := range stepName {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	if n < 1 {
		return 1
	}
	return n
}
