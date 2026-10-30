package responses

import (
	"encoding/json"
	"testing"

	"github.com/WncFht/devin2api/internal/llm"
)

// TestResponseMessagePhases reproduces progress -> tool -> result -> final turns.
// Codex must receive commentary on tool turns and final_answer on the last reply,
// consistently in item.done and response.completed, without buffering text deltas.
func TestResponseMessagePhases(t *testing.T) {
	for _, test := range []struct {
		name   string
		reason llm.StopReason
		tool   string
		phase  string
	}{
		{"tool_progress", llm.StopReasonToolUse, "function", "commentary"},
		{"custom_tool_progress", llm.StopReasonToolUse, "custom", "commentary"},
		{"tool_with_stop", llm.StopReasonStop, "function", "commentary"},
		{"final_answer", llm.StopReasonStop, "", "final_answer"},
		{"truncated", llm.StopReasonLength, "", "commentary"},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := &llm.AssistantMessage{Content: []llm.Content{llm.TextContent{Text: "unchanged text"}}, StopReason: test.reason}
			encoder := NewStreamEncoder("swe-2", nil)
			events := encodeStreamEvents(t, encoder, []llm.ResponseEvent{
				{Type: llm.ResponseEventStart, Partial: message},
				{Type: llm.ResponseEventTextStart, ContentIndex: 0, Partial: message},
				{Type: llm.ResponseEventTextDelta, ContentIndex: 0, Delta: "unchanged text", Partial: message},
				{Type: llm.ResponseEventTextEnd, ContentIndex: 0, Content: "unchanged text", Partial: message},
			})
			foundDelta := false
			for _, event := range events {
				foundDelta = foundDelta || event.Name == "response.output_text.delta"
				if event.Name == "response.output_item.done" {
					t.Fatal("message phase was finalized before a subsequent tool could arrive")
				}
			}
			if !foundDelta {
				t.Fatal("text deltas must be delivered before the response finishes")
			}
			if test.tool != "" {
				call := llm.ToolCall{ID: "call1", Name: "probe", Arguments: json.RawMessage(`{}`), Custom: test.tool == "custom"}
				message.Content = append(message.Content, call)
				events = append(events, encodeStreamEvents(t, encoder, []llm.ResponseEvent{
					{Type: llm.ResponseEventToolCallStart, ContentIndex: 1, ToolCallID: call.ID, ToolName: call.Name, Partial: message},
					{Type: llm.ResponseEventToolCallEnd, ContentIndex: 1, ToolCall: &call, Partial: message},
				})...)
			}
			events = append(events, encodeStreamEvents(t, encoder, []llm.ResponseEvent{{Type: llm.ResponseEventDone, Reason: test.reason, Message: message}})...)
			assertSequenceNumbers(t, events)
			checked := 0
			for _, event := range events {
				if event.Name != "response.output_item.done" {
					continue
				}
				item := decodeEventData(t, event)["item"].(map[string]any)
				if item["type"] == "message" {
					if item["phase"] != test.phase {
						t.Fatalf("phase = %v, want %s", item["phase"], test.phase)
					}
					checked++
				}
			}
			if checked != 1 {
				t.Fatalf("message item.done count = %d", checked)
			}
			var response map[string]any
			encoded, err := EncodeResponse(message, "swe-2", nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &response); err != nil {
				t.Fatal(err)
			}
			stream := decodeEventData(t, events[len(events)-1])["response"].(map[string]any)
			for _, body := range []map[string]any{response, stream} {
				for _, item := range body["output"].([]any) {
					if item := item.(map[string]any); item["type"] == "message" && item["phase"] != test.phase {
						t.Fatalf("completed phase = %v, want %s", item["phase"], test.phase)
					}
				}
			}
		})
	}
}

// TestResponseFailureKeepsProgressPhase prevents a failed stream becoming a final answer.
func TestResponseFailureKeepsProgressPhase(t *testing.T) {
	encoder := NewStreamEncoder("swe-2", nil)
	message := &llm.AssistantMessage{Content: []llm.Content{llm.TextContent{Text: "progress"}}, StopReason: llm.StopReasonPending}
	encodeStreamEvents(t, encoder, []llm.ResponseEvent{
		{Type: llm.ResponseEventTextStart, ContentIndex: 0, Partial: message},
		{Type: llm.ResponseEventTextEnd, ContentIndex: 0, Content: "progress", Partial: message},
	})
	events := encoder.failed(llm.ResponseEvent{Type: llm.ResponseEventError, Error: &llm.AssistantMessage{StopReason: llm.StopReasonError, ErrorMessage: "failed"}})
	item := decodeEventData(t, events[0])["item"].(map[string]any)
	if item["phase"] != "commentary" || events[len(events)-1].Name != "response.failed" {
		t.Fatalf("failed stream phase = %v", item["phase"])
	}
}
