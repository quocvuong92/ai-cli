package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/quocvuong92/ai-cli/internal/config"
	"github.com/quocvuong92/ai-cli/internal/constants"
	"github.com/quocvuong92/ai-cli/internal/logging"
)

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool represents a function/tool that the AI can call
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function definition
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ToolCall represents a function call from the AI
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Index    int    `json:"index,omitempty"` // Used in streaming responses
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ChatRequest represents the Chat Completions API request
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	Stream   bool      `json:"stream,omitempty"`
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Delta represents streaming delta content
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Choice represents a response choice
type Choice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta,omitempty"`
	Message      Message `json:"message,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

// HasToolCalls checks if the choice contains tool calls
func (c *Choice) HasToolCalls() bool {
	return len(c.Message.ToolCalls) > 0 || len(c.Delta.ToolCalls) > 0
}

// GetToolCalls extracts tool calls from the choice
func (c *Choice) GetToolCalls() []ToolCall {
	if len(c.Message.ToolCalls) > 0 {
		return c.Message.ToolCalls
	}
	return c.Delta.ToolCalls
}

// ChatResponse represents the API response
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// AzureErrorResponse represents an Azure API error
type AzureErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// APIError represents an error with status code
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

// AzureClient is the Azure OpenAI API client
type AzureClient struct {
	httpClient *http.Client
	config     *config.Config
	httpLogger *logging.HTTPLogger
}

// NewAzureClient creates a new Azure OpenAI client
func NewAzureClient(cfg *config.Config) *AzureClient {
	transport := http.DefaultTransport

	var httpLogger *logging.HTTPLogger
	if cfg.Debug {
		logger := logging.New(logging.Options{
			Level:  logging.LevelDebug,
			Format: logging.FormatJSON,
		})
		httpLogger = logging.NewHTTPLogger(logger)
		transport = logging.NewLoggingRoundTripper(http.DefaultTransport, httpLogger, true)
	}

	return &AzureClient{
		httpClient: &http.Client{
			Timeout:   constants.DefaultAPITimeout,
			Transport: transport,
		},
		config:     cfg,
		httpLogger: httpLogger,
	}
}

// Query sends a query to Azure OpenAI (non-streaming)
func (c *AzureClient) Query(systemPrompt, userMessage string) (*ChatResponse, error) {
	return c.QueryWithContext(context.Background(), systemPrompt, userMessage)
}

// QueryWithContext sends a query to Azure OpenAI with context support (non-streaming)
func (c *AzureClient) QueryWithContext(ctx context.Context, systemPrompt, userMessage string) (*ChatResponse, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return c.QueryWithHistoryContext(ctx, messages)
}

// QueryWithHistory sends a query with full message history (non-streaming)
func (c *AzureClient) QueryWithHistory(messages []Message) (*ChatResponse, error) {
	return c.QueryWithHistoryContext(context.Background(), messages)
}

// QueryWithHistoryContext sends a query with full message history and context support (non-streaming)
func (c *AzureClient) QueryWithHistoryContext(ctx context.Context, messages []Message) (*ChatResponse, error) {
	return c.QueryWithHistoryAndToolsContext(ctx, messages, nil)
}

// QueryWithHistoryAndToolsContext sends a query with full message history, tools, and context support (non-streaming)
func (c *AzureClient) QueryWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	reqBody := ChatRequest{
		Model:    c.config.Model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use retry logic for transient failures
	return WithRetry(ctx, func() (*ChatResponse, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.GetAzureAPIURL(), bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.config.AzureAPIKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			var errResp AzureErrorResponse
			errMsg := fmt.Sprintf("status code %d", resp.StatusCode)
			if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
				errMsg = errResp.Error.Message
			}
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("Azure API error: %s", errMsg),
			}
		}

		var chatResp ChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		return &chatResp, nil
	})
}

// QueryStream sends a streaming query to Azure OpenAI
func (c *AzureClient) QueryStream(systemPrompt, userMessage string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamWithContext(context.Background(), systemPrompt, userMessage, onChunk, onDone)
}

// QueryStreamWithContext sends a streaming query to Azure OpenAI with context support
func (c *AzureClient) QueryStreamWithContext(ctx context.Context, systemPrompt, userMessage string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return c.QueryStreamWithHistoryContext(ctx, messages, onChunk, onDone)
}

// QueryStreamWithHistory sends a streaming query with full message history
func (c *AzureClient) QueryStreamWithHistory(messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamWithHistoryContext(context.Background(), messages, onChunk, onDone)
}

// QueryStreamWithHistoryContext sends a streaming query with full message history and context support
func (c *AzureClient) QueryStreamWithHistoryContext(ctx context.Context, messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamWithHistoryAndToolsContext(ctx, messages, nil, onChunk, onDone)
}

// QueryStreamWithHistoryAndToolsContext sends a streaming query with full message history, tools, and context support
func (c *AzureClient) QueryStreamWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	reqBody := ChatRequest{
		Model:    c.config.Model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use retry logic for transient failures (before stream starts)
	return WithStreamRetry(ctx, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.GetAzureAPIURL(), bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Authorization", "Bearer "+c.config.AzureAPIKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			var errResp AzureErrorResponse
			errMsg := fmt.Sprintf("status code %d", resp.StatusCode)
			if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
				errMsg = errResp.Error.Message
			}
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("Azure API error: %s", errMsg),
			}
		}

		return resp, nil
	}, onChunk, onDone)
}

// Close is a no-op for AzureClient as it doesn't hold any resources
func (c *AzureClient) Close() {
	// No resources to clean up
}

// GetContent extracts the content from the response
func (r *ChatResponse) GetContent() string {
	if len(r.Choices) > 0 {
		if r.Choices[0].Message.Content != "" {
			return r.Choices[0].Message.Content
		}
		return r.Choices[0].Delta.Content
	}
	return ""
}

// GetUsageMap returns usage as a map for display
func (r *ChatResponse) GetUsageMap() map[string]int {
	return map[string]int{
		"input_tokens":  r.Usage.PromptTokens,
		"output_tokens": r.Usage.CompletionTokens,
		"total_tokens":  r.Usage.TotalTokens,
	}
}
