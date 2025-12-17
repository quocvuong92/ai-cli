package api

import (
	"context"
	"strings"
	"testing"
)

func TestSSEProcessor_Process_SimpleContent(t *testing.T) {
	input := `data: {"id":"test-1","choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: {"id":"test-1","choices":[{"index":0,"delta":{"content":" World"}}]}

data: [DONE]
`
	processor := NewSSEProcessor(strings.NewReader(input))

	var chunks []string
	err := processor.Process(context.Background(), func(content string) {
		chunks = append(chunks, content)
	})

	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	if len(chunks) != 2 {
		t.Errorf("Process() got %d chunks, want 2", len(chunks))
	}

	if processor.GetContent() != "Hello World" {
		t.Errorf("GetContent() = %q, want %q", processor.GetContent(), "Hello World")
	}
}

func TestSSEProcessor_Process_WithToolCalls(t *testing.T) {
	input := `data: {"id":"test-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"execute_command","arguments":"{\"com"}}]}}]}

data: {"id":"test-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"mand\":\"ls\"}"}}]}}]}

data: [DONE]
`
	processor := NewSSEProcessor(strings.NewReader(input))

	err := processor.Process(context.Background(), func(content string) {})

	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	if !processor.HasToolCalls() {
		t.Error("HasToolCalls() = false, want true")
	}
}

func TestSSEProcessor_BuildResponse(t *testing.T) {
	input := `data: {"id":"resp-123","choices":[{"index":0,"delta":{"content":"Test response"}}]}

data: {"id":"resp-123","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]
`
	processor := NewSSEProcessor(strings.NewReader(input))

	err := processor.Process(context.Background(), func(content string) {})
	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	resp := processor.BuildResponse()

	if resp.ID != "resp-123" {
		t.Errorf("BuildResponse().ID = %q, want %q", resp.ID, "resp-123")
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("BuildResponse().Choices length = %d, want 1", len(resp.Choices))
	}

	if resp.Choices[0].Message.Content != "Test response" {
		t.Errorf("BuildResponse().Choices[0].Message.Content = %q, want %q", resp.Choices[0].Message.Content, "Test response")
	}

	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("BuildResponse().Choices[0].FinishReason = %q, want %q", resp.Choices[0].FinishReason, "stop")
	}

	if resp.Usage.TotalTokens != 15 {
		t.Errorf("BuildResponse().Usage.TotalTokens = %d, want %d", resp.Usage.TotalTokens, 15)
	}
}

func TestSSEProcessor_Process_EmptyLines(t *testing.T) {
	input := `

data: {"id":"test-1","choices":[{"index":0,"delta":{"content":"Hello"}}]}



data: [DONE]

`
	processor := NewSSEProcessor(strings.NewReader(input))

	err := processor.Process(context.Background(), func(content string) {})

	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	if processor.GetContent() != "Hello" {
		t.Errorf("GetContent() = %q, want %q", processor.GetContent(), "Hello")
	}
}

func TestSSEProcessor_Process_ContextCancelled(t *testing.T) {
	input := `data: {"id":"test-1","choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: {"id":"test-1","choices":[{"index":0,"delta":{"content":" World"}}]}

data: [DONE]
`
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	processor := NewSSEProcessor(strings.NewReader(input))

	err := processor.Process(ctx, func(content string) {})

	if err == nil {
		t.Error("Process() expected error for cancelled context, got nil")
	}
}

func TestSSEProcessor_Process_InvalidJSON(t *testing.T) {
	input := `data: {"id":"test-1","choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: invalid json here

data: {"id":"test-1","choices":[{"index":0,"delta":{"content":" World"}}]}

data: [DONE]
`
	processor := NewSSEProcessor(strings.NewReader(input))

	err := processor.Process(context.Background(), func(content string) {})

	// Should not error, just skip invalid JSON
	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	// Should still have parsed the valid content
	if processor.GetContent() != "Hello World" {
		t.Errorf("GetContent() = %q, want %q", processor.GetContent(), "Hello World")
	}
}

func TestSSEProcessor_Process_NonDataLines(t *testing.T) {
	input := `event: message
id: 123
data: {"id":"test-1","choices":[{"index":0,"delta":{"content":"Hello"}}]}

retry: 3000

data: [DONE]
`
	processor := NewSSEProcessor(strings.NewReader(input))

	err := processor.Process(context.Background(), func(content string) {})

	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	if processor.GetContent() != "Hello" {
		t.Errorf("GetContent() = %q, want %q", processor.GetContent(), "Hello")
	}
}

func TestNewSSEProcessor(t *testing.T) {
	reader := strings.NewReader("")
	processor := NewSSEProcessor(reader)

	if processor == nil {
		t.Error("NewSSEProcessor() returned nil")
	}

	if processor.reader == nil {
		t.Error("NewSSEProcessor().reader is nil")
	}

	if processor.toolCallsMap == nil {
		t.Error("NewSSEProcessor().toolCallsMap is nil")
	}
}
