// Package stream defines the Matrix Agent session streaming contract on top of
// the AG-UI event protocol.
//
// # Sink contract
//
// A [Sink] receives standard AG-UI events ([agui.Event]) produced during a TAOR
// session (query loop), tool execution, or coordinator worker run. Implementations
// must treat [Sink.Publish] as potentially blocking until the event is consumed or
// ctx is cancelled.
//
// Event builders in this package (RunStarted, TextMessageContent, ToolCallStart,
// ToolOutputDelta, etc.) emit protocol-compliant events. Callers attach session
// lineage via [Meta] and [WithMeta]; [ScopeSink] tags worker-scoped streams.
//
// [CoalesceSink] and [OutputCoalesceSink] are optional wrappers that batch high-
// frequency text/reasoning and tool-output deltas before forwarding to an inner
// sink (e.g. RunView projector or SSE bridge).
package stream
