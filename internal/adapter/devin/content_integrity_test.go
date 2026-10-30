package devin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	devinproto "local/devinproto"

	"github.com/WncFht/devin2api/internal/llm"
)

// TestStreamPreservesCallerText checks the actual Connect wire, not a bypassed helper.
func TestStreamPreservesCallerText(t *testing.T) {
	const phrase = "You are Claude Code, Anthropic's official CLI for Claude."
	system := "SYSTEM_LITERAL " + phrase
	description := "TOOL_LITERAL PROBE_FREEFORM_OK " + phrase
	user := "USER_LITERAL " + phrase
	history := "HISTORY_LITERAL " + phrase
	result := "RESULT_LITERAL " + phrase
	args, err := json.Marshal(map[string]string{"text": history})
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubUpstream{
		catalog: []*devinproto.ExaCodeiumCommonPb_ClientModelConfig{stubModelEntry("stub-model", false)},
		chat: func(_ int, req *devinproto.GetChatMessageRequest, stream *connect.ServerStream[devinproto.GetChatMessageResponse]) error {
			if !strings.Contains(req.GetPrompt(), system) || !strings.Contains(req.GetPrompt(), description) {
				t.Error("system prompt or tool description was rewritten")
			}
			for _, want := range []string{user, result} {
				found := false
				for _, prompt := range req.GetChatMessagePrompts() {
					found = found || strings.Contains(prompt.GetPrompt(), want)
				}
				if !found {
					t.Errorf("message literal missing: %q", want)
				}
			}
			foundArgs := false
			for _, prompt := range req.GetChatMessagePrompts() {
				for _, call := range prompt.GetToolCalls() {
					foundArgs = foundArgs || call.GetArgumentsJson() == string(args)
				}
			}
			if !foundArgs {
				t.Error("historical tool arguments were rewritten")
			}
			return stubSend(stream, stubMeta(), stubDelta("ok"), stubStop())
		},
	}
	adapter := stubAdapter(t, stubServer(t, stub, nil), Config{Model: "stub-model", Identity: LaneIdentity{Token: "test-token"}})
	request := llm.RequestMessages{
		SystemPrompt: system,
		Tools:        []llm.ToolDefinition{{Name: "probe", Description: description, InputSchema: json.RawMessage(`{"type":"object"}`)}},
		Messages: []llm.Message{
			llm.UserMessage{Content: []llm.Content{llm.TextContent{Text: user}}},
			llm.AssistantMessage{Content: []llm.Content{llm.ToolCall{ID: "call-1", Name: "probe", Arguments: args}}},
			llm.ToolResultMessage{ToolCallID: "call-1", Content: []llm.Content{llm.TextContent{Text: result}}},
			llm.UserMessage{Content: []llm.Content{llm.TextContent{Text: "continue"}}},
		},
	}
	before, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := adapter.Stream(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	stubDrain(t, stream)
	after, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("adapter mutated caller-owned prompt, tools or history")
	}
}

// TestStreamPreservesUpstreamPolicyError requires a rejection rather than rewritten retry.
func TestStreamPreservesUpstreamPolicyError(t *testing.T) {
	const message = "upstream policy denied ORIGINAL_FREEFORM"
	stub := &stubUpstream{
		catalog: []*devinproto.ExaCodeiumCommonPb_ClientModelConfig{stubModelEntry("stub-model", false)},
		chat: func(_ int, _ *devinproto.GetChatMessageRequest, _ *connect.ServerStream[devinproto.GetChatMessageResponse]) error {
			return connect.NewError(connect.CodePermissionDenied, errors.New(message))
		},
	}
	adapter := stubAdapter(t, stubServer(t, stub, nil), Config{Model: "stub-model", Identity: LaneIdentity{Token: "test-token"}})
	stream, err := adapter.Stream(context.Background(), stubRequest())
	var rejection string
	if err == nil {
		for err == nil {
			var event llm.ResponseEvent
			event, err = stream.Recv(context.Background())
			if event.Type == llm.ResponseEventError && event.Error != nil {
				rejection = event.Error.ErrorMessage
			}
		}
	} else {
		rejection = err.Error()
	}
	if !strings.Contains(rejection, message) {
		t.Fatalf("upstream rejection lost: %s (%v)", rejection, err)
	}
	if stub.chatCalls.Load() != 1 {
		t.Fatalf("policy rejection retried: %d calls", stub.chatCalls.Load())
	}
}
