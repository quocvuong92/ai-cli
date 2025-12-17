package api

import (
	"testing"
)

func TestChatResponse_GetContent(t *testing.T) {
	tests := []struct {
		name     string
		response ChatResponse
		want     string
	}{
		{
			name: "content in message",
			response: ChatResponse{
				Choices: []Choice{
					{Message: Message{Content: "Hello World"}},
				},
			},
			want: "Hello World",
		},
		{
			name: "content in delta",
			response: ChatResponse{
				Choices: []Choice{
					{Delta: Delta{Content: "Hello Delta"}},
				},
			},
			want: "Hello Delta",
		},
		{
			name: "message takes precedence",
			response: ChatResponse{
				Choices: []Choice{
					{
						Message: Message{Content: "Message Content"},
						Delta:   Delta{Content: "Delta Content"},
					},
				},
			},
			want: "Message Content",
		},
		{
			name:     "empty choices",
			response: ChatResponse{Choices: []Choice{}},
			want:     "",
		},
		{
			name:     "nil choices",
			response: ChatResponse{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.GetContent()
			if got != tt.want {
				t.Errorf("GetContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChatResponse_GetUsageMap(t *testing.T) {
	resp := ChatResponse{
		Usage: Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	got := resp.GetUsageMap()

	if got["input_tokens"] != 100 {
		t.Errorf("GetUsageMap()[\"input_tokens\"] = %d, want 100", got["input_tokens"])
	}
	if got["output_tokens"] != 50 {
		t.Errorf("GetUsageMap()[\"output_tokens\"] = %d, want 50", got["output_tokens"])
	}
	if got["total_tokens"] != 150 {
		t.Errorf("GetUsageMap()[\"total_tokens\"] = %d, want 150", got["total_tokens"])
	}
}

func TestChoice_HasToolCalls(t *testing.T) {
	tests := []struct {
		name   string
		choice Choice
		want   bool
	}{
		{
			name: "tool calls in message",
			choice: Choice{
				Message: Message{
					ToolCalls: []ToolCall{{ID: "call_1"}},
				},
			},
			want: true,
		},
		{
			name: "tool calls in delta",
			choice: Choice{
				Delta: Delta{
					ToolCalls: []ToolCall{{ID: "call_2"}},
				},
			},
			want: true,
		},
		{
			name:   "no tool calls",
			choice: Choice{},
			want:   false,
		},
		{
			name: "empty tool calls in message",
			choice: Choice{
				Message: Message{ToolCalls: []ToolCall{}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.choice.HasToolCalls()
			if got != tt.want {
				t.Errorf("HasToolCalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChoice_GetToolCalls(t *testing.T) {
	tests := []struct {
		name   string
		choice Choice
		want   int
	}{
		{
			name: "tool calls in message",
			choice: Choice{
				Message: Message{
					ToolCalls: []ToolCall{{ID: "call_1"}, {ID: "call_2"}},
				},
			},
			want: 2,
		},
		{
			name: "tool calls in delta",
			choice: Choice{
				Delta: Delta{
					ToolCalls: []ToolCall{{ID: "call_3"}},
				},
			},
			want: 1,
		},
		{
			name: "message takes precedence",
			choice: Choice{
				Message: Message{
					ToolCalls: []ToolCall{{ID: "msg_call"}},
				},
				Delta: Delta{
					ToolCalls: []ToolCall{{ID: "delta_call"}},
				},
			},
			want: 1, // Returns message tool calls
		},
		{
			name:   "no tool calls",
			choice: Choice{},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.choice.GetToolCalls()
			if len(got) != tt.want {
				t.Errorf("GetToolCalls() returned %d items, want %d", len(got), tt.want)
			}
		})
	}
}

func TestMessage_ToolCallID(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    "result",
		ToolCallID: "call_123",
	}

	if msg.ToolCallID != "call_123" {
		t.Errorf("Message.ToolCallID = %q, want %q", msg.ToolCallID, "call_123")
	}
}

func TestToolCall_Structure(t *testing.T) {
	tc := ToolCall{
		ID:    "call_abc123",
		Type:  "function",
		Index: 0,
	}
	tc.Function.Name = "execute_command"
	tc.Function.Arguments = `{"command":"ls -la"}`

	if tc.ID != "call_abc123" {
		t.Errorf("ToolCall.ID = %q, want %q", tc.ID, "call_abc123")
	}
	if tc.Type != "function" {
		t.Errorf("ToolCall.Type = %q, want %q", tc.Type, "function")
	}
	if tc.Function.Name != "execute_command" {
		t.Errorf("ToolCall.Function.Name = %q, want %q", tc.Function.Name, "execute_command")
	}
}
