package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/quocvuong92/ai-cli/internal/api"
	"github.com/quocvuong92/ai-cli/internal/config"
	"github.com/quocvuong92/ai-cli/internal/executor"
)

// MockAIClient implements api.AIClient for testing
type MockAIClient struct {
	responses     []api.ChatResponse
	responseIndex int
	lastMessages  []api.Message
	lastTools     []api.Tool
	streamChunks  []string
	shouldError   error
}

func NewMockAIClient() *MockAIClient {
	return &MockAIClient{
		responses: []api.ChatResponse{},
	}
}

func (m *MockAIClient) SetResponse(content string) {
	m.responses = []api.ChatResponse{{
		ID: "test-response",
		Choices: []api.Choice{{
			Index: 0,
			Message: api.Message{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: "stop",
		}},
		Usage: api.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}}
}

func (m *MockAIClient) SetToolCallResponse(toolName string, args string) {
	m.responses = []api.ChatResponse{{
		ID: "test-response",
		Choices: []api.Choice{{
			Index: 0,
			Message: api.Message{
				Role: "assistant",
				ToolCalls: []api.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      toolName,
						Arguments: args,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}}
}

func (m *MockAIClient) AddResponse(resp api.ChatResponse) {
	m.responses = append(m.responses, resp)
}

func (m *MockAIClient) SetStreamChunks(chunks []string) {
	m.streamChunks = chunks
}

func (m *MockAIClient) SetError(err error) {
	m.shouldError = err
}

func (m *MockAIClient) getNextResponse() (*api.ChatResponse, error) {
	if m.shouldError != nil {
		return nil, m.shouldError
	}
	if m.responseIndex >= len(m.responses) {
		// Return empty response if no more responses configured
		return &api.ChatResponse{
			ID: "empty",
			Choices: []api.Choice{{
				Message: api.Message{Role: "assistant", Content: ""},
			}},
		}, nil
	}
	resp := m.responses[m.responseIndex]
	m.responseIndex++
	return &resp, nil
}

func (m *MockAIClient) Query(systemPrompt, userMessage string) (*api.ChatResponse, error) {
	return m.QueryWithContext(context.Background(), systemPrompt, userMessage)
}

func (m *MockAIClient) QueryWithContext(ctx context.Context, systemPrompt, userMessage string) (*api.ChatResponse, error) {
	m.lastMessages = []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return m.getNextResponse()
}

func (m *MockAIClient) QueryWithHistory(messages []api.Message) (*api.ChatResponse, error) {
	return m.QueryWithHistoryContext(context.Background(), messages)
}

func (m *MockAIClient) QueryWithHistoryContext(ctx context.Context, messages []api.Message) (*api.ChatResponse, error) {
	m.lastMessages = messages
	return m.getNextResponse()
}

func (m *MockAIClient) QueryWithHistoryAndToolsContext(ctx context.Context, messages []api.Message, tools []api.Tool) (*api.ChatResponse, error) {
	m.lastMessages = messages
	m.lastTools = tools
	return m.getNextResponse()
}

func (m *MockAIClient) QueryStream(systemPrompt, userMessage string, onChunk func(string), onDone func(*api.ChatResponse)) error {
	return m.QueryStreamWithContext(context.Background(), systemPrompt, userMessage, onChunk, onDone)
}

func (m *MockAIClient) QueryStreamWithContext(ctx context.Context, systemPrompt, userMessage string, onChunk func(string), onDone func(*api.ChatResponse)) error {
	m.lastMessages = []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return m.streamResponse(onChunk, onDone)
}

func (m *MockAIClient) QueryStreamWithHistory(messages []api.Message, onChunk func(string), onDone func(*api.ChatResponse)) error {
	return m.QueryStreamWithHistoryContext(context.Background(), messages, onChunk, onDone)
}

func (m *MockAIClient) QueryStreamWithHistoryContext(ctx context.Context, messages []api.Message, onChunk func(string), onDone func(*api.ChatResponse)) error {
	m.lastMessages = messages
	return m.streamResponse(onChunk, onDone)
}

func (m *MockAIClient) QueryStreamWithHistoryAndToolsContext(ctx context.Context, messages []api.Message, tools []api.Tool, onChunk func(string), onDone func(*api.ChatResponse)) error {
	m.lastMessages = messages
	m.lastTools = tools
	return m.streamResponse(onChunk, onDone)
}

func (m *MockAIClient) streamResponse(onChunk func(string), onDone func(*api.ChatResponse)) error {
	if m.shouldError != nil {
		return m.shouldError
	}

	// Send stream chunks if configured
	for _, chunk := range m.streamChunks {
		onChunk(chunk)
	}

	// Call onDone with the response
	resp, _ := m.getNextResponse()
	if onDone != nil && resp != nil {
		onDone(resp)
	}
	return nil
}

func (m *MockAIClient) Close() {}

// Ensure MockAIClient implements api.AIClient
var _ api.AIClient = (*MockAIClient)(nil)

// TestIntegration_QueryWithToolExecution tests a full workflow:
// user query → AI responds with tool call → tool executes → AI gives final response
func TestIntegration_QueryWithToolExecution(t *testing.T) {
	// Setup
	app := newTestApp()
	mockClient := NewMockAIClient()
	exec := executor.NewExecutor()

	// Create a test file to read
	dir := createTestDir(t)
	testFile := createTestFile(t, dir, "test.txt", "Hello from test file!")

	// Configure mock responses:
	// 1. First response: AI requests to read the file
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-1",
		Choices: []api.Choice{{
			Message: api.Message{
				Role: "assistant",
				ToolCalls: []api.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "read_file",
						Arguments: `{"path": "` + testFile + `"}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	})

	// 2. Second response: AI provides final answer after seeing file content
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-2",
		Choices: []api.Choice{{
			Message: api.Message{
				Role:    "assistant",
				Content: "The file contains: Hello from test file!",
			},
			FinishReason: "stop",
		}},
	})

	// Create session
	session := &InteractiveSession{
		app:    app,
		client: mockClient,
		exec:   exec,
		messages: []api.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What's in the test file?"},
		},
		interruptCtx: NewInterruptibleContext(),
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute
	response, err := app.sendInteractiveMessageWithTools(mockClient, exec, &session.messages, session.interruptCtx, session)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Verify
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if response != "The file contains: Hello from test file!" {
		t.Errorf("Expected final response about file content, got: %s", response)
	}

	// Verify the conversation flow
	// Messages should include: system, user, assistant (tool call), tool result, assistant (final)
	if len(session.messages) < 4 {
		t.Errorf("Expected at least 4 messages in conversation, got %d", len(session.messages))
	}

	// Verify tool result was added
	foundToolResult := false
	for _, msg := range session.messages {
		if msg.Role == "tool" && strings.Contains(msg.Content, "Hello from test file!") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Error("Expected tool result message with file content")
	}
}

// TestIntegration_MultipleToolCalls tests a workflow with multiple tool calls
func TestIntegration_MultipleToolCalls(t *testing.T) {
	app := newTestApp()
	mockClient := NewMockAIClient()
	exec := executor.NewExecutor()

	dir := createTestDir(t)
	createTestFile(t, dir, "file1.txt", "Content of file 1")
	createTestFile(t, dir, "file2.txt", "Content of file 2")

	// First response: list directory
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-1",
		Choices: []api.Choice{{
			Message: api.Message{
				Role: "assistant",
				ToolCalls: []api.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "list_directory",
						Arguments: `{"path": "` + dir + `"}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	})

	// Second response: final answer
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-2",
		Choices: []api.Choice{{
			Message: api.Message{
				Role:    "assistant",
				Content: "Found 2 files: file1.txt and file2.txt",
			},
			FinishReason: "stop",
		}},
	})

	session := &InteractiveSession{
		app:    app,
		client: mockClient,
		exec:   exec,
		messages: []api.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "List the files in the test directory"},
		},
		interruptCtx: NewInterruptibleContext(),
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	response, err := app.sendInteractiveMessageWithTools(mockClient, exec, &session.messages, session.interruptCtx, session)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(response, "Found 2 files") {
		t.Errorf("Expected response about found files, got: %s", response)
	}

	// Verify tool result contains the file names
	foundToolResult := false
	for _, msg := range session.messages {
		if msg.Role == "tool" {
			if strings.Contains(msg.Content, "file1.txt") && strings.Contains(msg.Content, "file2.txt") {
				foundToolResult = true
				break
			}
		}
	}
	if !foundToolResult {
		t.Error("Expected tool result to contain both file names")
	}
}

// TestIntegration_SearchFilesTool tests the search_files tool integration
func TestIntegration_SearchFilesTool(t *testing.T) {
	app := newTestApp()
	mockClient := NewMockAIClient()
	exec := executor.NewExecutor()

	dir := createTestDir(t)
	createTestFile(t, dir, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}")
	createTestFile(t, dir, "utils.go", "package main\n\nfunc helper() {}")

	// First response: search for function
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-1",
		Choices: []api.Choice{{
			Message: api.Message{
				Role: "assistant",
				ToolCalls: []api.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "search_files",
						Arguments: `{"pattern": "func main", "path": "` + dir + `"}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	})

	// Second response: final answer
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-2",
		Choices: []api.Choice{{
			Message: api.Message{
				Role:    "assistant",
				Content: "Found the main function in main.go",
			},
			FinishReason: "stop",
		}},
	})

	session := &InteractiveSession{
		app:    app,
		client: mockClient,
		exec:   exec,
		messages: []api.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Find the main function"},
		},
		interruptCtx: NewInterruptibleContext(),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	response, err := app.sendInteractiveMessageWithTools(mockClient, exec, &session.messages, session.interruptCtx, session)

	w.Close()
	os.Stdout = oldStdout
	io.Copy(&bytes.Buffer{}, r)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(response, "main.go") {
		t.Errorf("Expected response to mention main.go, got: %s", response)
	}
}

// TestIntegration_SimpleQueryNoTools tests a simple query without tool calls
func TestIntegration_SimpleQueryNoTools(t *testing.T) {
	app := newTestApp()
	app.cfg.Stream = false
	mockClient := NewMockAIClient()

	mockClient.SetResponse("The capital of France is Paris.")

	messages := []api.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What is the capital of France?"},
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	response, err := app.sendInteractiveMessage(mockClient, messages)

	w.Close()
	os.Stdout = oldStdout
	io.Copy(&bytes.Buffer{}, r)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if response != "The capital of France is Paris." {
		t.Errorf("Expected response about Paris, got: %s", response)
	}
}

// TestIntegration_StreamingResponse tests streaming response handling
func TestIntegration_StreamingResponse(t *testing.T) {
	app := newTestApp()
	app.cfg.Stream = true
	app.cfg.Render = false
	mockClient := NewMockAIClient()

	// Configure streaming chunks - these are printed during streaming
	mockClient.SetStreamChunks([]string{"Hello", " ", "World", "!"})
	// The response content comes from the final response after streaming
	mockClient.SetResponse("Hello World!")

	messages := []api.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Say hello"},
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	response, err := app.sendInteractiveMessage(mockClient, messages)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Output should contain the streamed chunks (printed during streaming)
	output := buf.String()
	if !strings.Contains(output, "Hello") {
		t.Errorf("Expected output to contain streamed chunks, got: %s", output)
	}
	if !strings.Contains(output, "World") {
		t.Errorf("Expected output to contain 'World', got: %s", output)
	}

	// Response is the accumulated content - in streaming mode with non-render,
	// the content is printed directly and the final response is built from chunks
	// For this mock, we just verify no error occurred
	_ = response
}

// TestIntegration_ConfigDebugMode tests that debug mode is properly configured
func TestIntegration_ConfigDebugMode(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Debug = true

	if !cfg.Debug {
		t.Error("Expected Debug to be true")
	}

	// Verify that the config can be validated with debug mode
	cfg.Provider = "azure"
	cfg.AzureEndpoint = "https://test.openai.azure.com"
	cfg.AzureAPIKey = "test-key"
	cfg.Model = "gpt-4"

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Config validation failed with debug mode: %v", err)
	}
}

// TestIntegration_ToolsAvailable tests that default tools are properly defined
func TestIntegration_ToolsAvailable(t *testing.T) {
	tools := api.GetDefaultTools()

	expectedTools := []string{
		"execute_command",
		"read_file",
		"write_file",
		"edit_file",
		"delete_file",
		"search_files",
		"list_directory",
		"update_plan",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Function.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool %q not found in default tools", expected)
		}
	}
}

// TestIntegration_BlockedCommandExecution tests that dangerous commands are blocked
func TestIntegration_BlockedCommandExecution(t *testing.T) {
	app := newTestApp()
	mockClient := NewMockAIClient()
	exec := executor.NewExecutor()
	exec.GetPermissionManager().DisableDangerous()

	// AI tries to execute a dangerous command
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-1",
		Choices: []api.Choice{{
			Message: api.Message{
				Role: "assistant",
				ToolCalls: []api.ToolCall{{
					ID:   "call-1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "execute_command",
						Arguments: `{"command": "rm -rf /", "reasoning": "clean up"}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	})

	// After tool execution fails, AI apologizes
	mockClient.AddResponse(api.ChatResponse{
		ID: "resp-2",
		Choices: []api.Choice{{
			Message: api.Message{
				Role:    "assistant",
				Content: "I apologize, that command was blocked for safety.",
			},
			FinishReason: "stop",
		}},
	})

	session := &InteractiveSession{
		app:    app,
		client: mockClient,
		exec:   exec,
		messages: []api.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Delete everything"},
		},
		interruptCtx: NewInterruptibleContext(),
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_, err := app.sendInteractiveMessageWithTools(mockClient, exec, &session.messages, session.interruptCtx, session)

	w.Close()
	os.Stdout = oldStdout
	io.Copy(&bytes.Buffer{}, r)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the tool result contains "blocked"
	foundBlocked := false
	for _, msg := range session.messages {
		if msg.Role == "tool" && strings.Contains(strings.ToLower(msg.Content), "blocked") {
			foundBlocked = true
			break
		}
	}
	if !foundBlocked {
		t.Error("Expected dangerous command to be blocked")
	}
}
