package stream

import (
	"encoding/json"
	"fmt"

	agui "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

const (
	ActivityTypeSubagent      = "subagent"
	CustomNameToolOutputDelta = "tool_output_delta"
)

func NewMessageID() string { return agui.GenerateMessageID() }

func EventType(ev Event) string {
	if ev == nil {
		return ""
	}
	return string(ev.Type())
}

func RunStarted(meta Meta) Event {
	return agui.NewRunStartedEvent(meta.ThreadID, meta.RunID)
}

func RunFinished(meta Meta, result map[string]any) Event {
	return agui.NewRunFinishedEventWithOptions(meta.ThreadID, meta.RunID, agui.WithResult(result))
}

func RunError(meta Meta, message string) Event {
	return agui.NewRunErrorEvent(message, agui.WithRunID(meta.RunID))
}

func StepStarted(turn int) Event  { return agui.NewStepStartedEvent(turnStepName(turn)) }
func StepFinished(turn int) Event { return agui.NewStepFinishedEvent(turnStepName(turn)) }

func turnStepName(turn int) string {
	if turn < 1 {
		turn = 1
	}
	return fmt.Sprintf("turn-%d", turn)
}

func ReasoningStart(messageID string) Event { return agui.NewReasoningStartEvent(messageID) }
func ReasoningEnd(messageID string) Event   { return agui.NewReasoningEndEvent(messageID) }
func ReasoningMessageStart(messageID string) Event {
	return agui.NewReasoningMessageStartEvent(messageID, "assistant")
}
func ReasoningMessageContent(messageID, delta string) Event {
	return agui.NewReasoningMessageContentEvent(messageID, delta)
}
func ReasoningMessageEnd(messageID string) Event { return agui.NewReasoningMessageEndEvent(messageID) }

func TextMessageStart(messageID, name string) Event {
	opts := []agui.TextMessageStartOption{agui.WithRole("assistant")}
	if name != "" {
		opts = append(opts, agui.WithName(name))
	}
	return agui.NewTextMessageStartEvent(messageID, opts...)
}
func TextMessageContent(messageID, delta string) Event {
	return agui.NewTextMessageContentEvent(messageID, delta)
}
func TextMessageEnd(messageID string) Event { return agui.NewTextMessageEndEvent(messageID) }

func ToolCallStart(toolCallID, toolCallName string) Event {
	return agui.NewToolCallStartEvent(toolCallID, toolCallName)
}
func ToolCallArgs(toolCallID, delta string) Event {
	return agui.NewToolCallArgsEvent(toolCallID, delta)
}
func ToolCallEnd(toolCallID string) Event { return agui.NewToolCallEndEvent(toolCallID) }
func ToolCallResult(messageID, toolCallID, content string) Event {
	return agui.NewToolCallResultEvent(messageID, toolCallID, content)
}

func ToolOutputDelta(toolCallID, toolName, delta string) Event {
	return agui.NewCustomEvent(CustomNameToolOutputDelta, agui.WithValue(map[string]any{
		"toolCallId": toolCallID,
		"toolName":   toolName,
		"delta":      delta,
	}))
}

func SubagentActivity(snap any) Event {
	return agui.NewActivitySnapshotEvent(snapshotMessageID(snap), ActivityTypeSubagent, snap)
}

func snapshotMessageID(snap any) string {
	if snap == nil {
		return NewMessageID()
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return NewMessageID()
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return NewMessageID()
	}
	var id string
	if raw, ok := fields["id"]; ok {
		_ = json.Unmarshal(raw, &id)
	}
	if id == "" {
		return NewMessageID()
	}
	return id
}

func IsTextContent(ev Event) bool {
	if ev == nil {
		return false
	}
	switch ev.Type() {
	case agui.EventTypeTextMessageContent, agui.EventTypeReasoningMessageContent:
		return true
	default:
		return false
	}
}

func IsToolOutputDelta(ev Event) bool {
	ce, ok := ev.(*agui.CustomEvent)
	return ok && ce.Name == CustomNameToolOutputDelta
}

func ToolOutputDeltaFields(ev Event) (toolCallID, toolName, delta string, ok bool) {
	ce, ok := ev.(*agui.CustomEvent)
	if !ok || ce.Name != CustomNameToolOutputDelta {
		return "", "", "", false
	}
	m, ok := ce.Value.(map[string]any)
	if !ok {
		return "", "", "", false
	}
	toolCallID, _ = m["toolCallId"].(string)
	toolName, _ = m["toolName"].(string)
	delta, _ = m["delta"].(string)
	return toolCallID, toolName, delta, toolCallID != "" && delta != ""
}

func AppendTextDelta(ev Event, chunk string) Event {
	if chunk == "" || ev == nil {
		return ev
	}
	switch e := ev.(type) {
	case *agui.TextMessageContentEvent:
		return agui.NewTextMessageContentEvent(e.MessageID, e.Delta+chunk)
	case *agui.ReasoningMessageContentEvent:
		return agui.NewReasoningMessageContentEvent(e.MessageID, e.Delta+chunk)
	default:
		return ev
	}
}

func MergeToolOutputDelta(existing, incoming Event) Event {
	id1, name1, d1, ok1 := ToolOutputDeltaFields(existing)
	id2, name2, d2, ok2 := ToolOutputDeltaFields(incoming)
	if !ok1 {
		if ok2 {
			return incoming
		}
		return existing
	}
	if !ok2 {
		return existing
	}
	if id2 != id1 {
		return existing
	}
	if name1 == "" {
		name1 = name2
	}
	return ToolOutputDelta(id1, name1, d1+d2)
}
