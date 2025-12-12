package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/quocvuong92/ai-cli/internal/auth"
	"github.com/quocvuong92/ai-cli/internal/config"
)

// CopilotClient is the GitHub Copilot API client
type CopilotClient struct {
	httpClient   *http.Client
	config       *config.Config
	tokenManager *auth.TokenManager
}

// NewCopilotClient creates a new GitHub Copilot client
func NewCopilotClient(cfg *config.Config, tokenManager *auth.TokenManager) *CopilotClient {
	return &CopilotClient{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		config:       cfg,
		tokenManager: tokenManager,
	}
}

// getBaseURL returns the Copilot API base URL
func (c *CopilotClient) getBaseURL() string {
	return auth.GetCopilotBaseURL(c.config.AccountType)
}

// buildHeaders builds the required headers for Copilot API requests
func (c *CopilotClient) buildHeaders(ctx context.Context, enableVision bool) (map[string]string, error) {
	token, err := c.tokenManager.GetCopilotToken(ctx)
	if err != nil {
		return nil, err
	}

	headers := auth.BuildCopilotHeaders(token, enableVision)
	headers["X-Request-Id"] = uuid.New().String()

	return headers, nil
}

// Query sends a query to Copilot (non-streaming)
func (c *CopilotClient) Query(systemPrompt, userMessage string) (*ChatResponse, error) {
	return c.QueryWithContext(context.Background(), systemPrompt, userMessage)
}

// QueryWithContext sends a query to Copilot with context support (non-streaming)
func (c *CopilotClient) QueryWithContext(ctx context.Context, systemPrompt, userMessage string) (*ChatResponse, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return c.QueryWithHistoryContext(ctx, messages)
}

// QueryWithHistory sends a query with full message history (non-streaming)
func (c *CopilotClient) QueryWithHistory(messages []Message) (*ChatResponse, error) {
	return c.QueryWithHistoryContext(context.Background(), messages)
}

// QueryWithHistoryContext sends a query with full message history and context support (non-streaming)
func (c *CopilotClient) QueryWithHistoryContext(ctx context.Context, messages []Message) (*ChatResponse, error) {
	return c.QueryWithHistoryAndToolsContext(ctx, messages, nil)
}

// QueryWithHistoryAndToolsContext sends a query with full message history, tools, and context support (non-streaming)
func (c *CopilotClient) QueryWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
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

	headers, err := c.buildHeaders(ctx, false)
	if err != nil {
		return nil, err
	}

	// Set X-Initiator based on message roles
	hasAgentMessages := false
	for _, msg := range messages {
		if msg.Role == "assistant" || msg.Role == "tool" {
			hasAgentMessages = true
			break
		}
	}
	if hasAgentMessages {
		headers["X-Initiator"] = "agent"
	} else {
		headers["X-Initiator"] = "user"
	}

	url := c.getBaseURL() + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

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
		return nil, c.handleError(resp.StatusCode, body)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

// QueryStream sends a streaming query to Copilot
func (c *CopilotClient) QueryStream(systemPrompt, userMessage string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamWithContext(context.Background(), systemPrompt, userMessage, onChunk, onDone)
}

// QueryStreamWithContext sends a streaming query to Copilot with context support
func (c *CopilotClient) QueryStreamWithContext(ctx context.Context, systemPrompt, userMessage string, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}
	return c.QueryStreamWithHistoryContext(ctx, messages, onChunk, onDone)
}

// QueryStreamWithHistory sends a streaming query with full message history
func (c *CopilotClient) QueryStreamWithHistory(messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamWithHistoryContext(context.Background(), messages, onChunk, onDone)
}

// QueryStreamWithHistoryContext sends a streaming query with full message history and context support
func (c *CopilotClient) QueryStreamWithHistoryContext(ctx context.Context, messages []Message, onChunk func(content string), onDone func(resp *ChatResponse)) error {
	return c.QueryStreamWithHistoryAndToolsContext(ctx, messages, nil, onChunk, onDone)
}

// QueryStreamWithHistoryAndToolsContext sends a streaming query with full message history, tools, and context support
func (c *CopilotClient) QueryStreamWithHistoryAndToolsContext(ctx context.Context, messages []Message, tools []Tool, onChunk func(content string), onDone func(resp *ChatResponse)) error {
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

	headers, err := c.buildHeaders(ctx, false)
	if err != nil {
		return err
	}

	// Set X-Initiator based on message roles
	hasAgentMessages := false
	for _, msg := range messages {
		if msg.Role == "assistant" || msg.Role == "tool" {
			hasAgentMessages = true
			break
		}
	}
	if hasAgentMessages {
		headers["X-Initiator"] = "agent"
	} else {
		headers["X-Initiator"] = "user"
	}

	url := c.getBaseURL() + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.handleError(resp.StatusCode, body)
	}

	// Accumulate content and tool calls from streaming chunks
	var contentBuilder strings.Builder
	toolCallsMap := make(map[int]*ToolCall) // Index -> ToolCall for accumulating chunks
	var finalUsage Usage
	var responseID string

	reader := bufio.NewReader(resp.Body)

	for {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("request cancelled: %w", err)
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("Failed to parse streaming chunk: %v (data: %s)", err, data)
			continue
		}

		// Capture response ID
		if chunk.ID != "" {
			responseID = chunk.ID
		}

		// Send content chunk and accumulate
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			contentBuilder.WriteString(content)
			onChunk(content)
		}

		// Accumulate tool calls from streaming chunks
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			for _, tc := range chunk.Choices[0].Delta.ToolCalls {
				idx := tc.Index
				if existing, ok := toolCallsMap[idx]; ok {
					// Append to existing tool call
					existing.Function.Arguments += tc.Function.Arguments
				} else {
					// Create new tool call entry
					toolCallsMap[idx] = &ToolCall{
						ID:    tc.ID,
						Type:  tc.Type,
						Index: idx,
					}
					toolCallsMap[idx].Function.Name = tc.Function.Name
					toolCallsMap[idx].Function.Arguments = tc.Function.Arguments
				}
			}
		}

		// Capture usage from final chunk
		if chunk.Usage.TotalTokens > 0 {
			finalUsage = chunk.Usage
		}
	}

	// Build the final response with accumulated content and tool calls
	if onDone != nil {
		var toolCalls []ToolCall
		for i := 0; i < len(toolCallsMap); i++ {
			if tc, ok := toolCallsMap[i]; ok {
				toolCalls = append(toolCalls, *tc)
			}
		}

		finalResp := &ChatResponse{
			ID: responseID,
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role:      "assistant",
						Content:   contentBuilder.String(),
						ToolCalls: toolCalls,
					},
					FinishReason: "stop",
				},
			},
			Usage: finalUsage,
		}
		if len(toolCalls) > 0 {
			finalResp.Choices[0].FinishReason = "tool_calls"
		}
		onDone(finalResp)
	}

	return nil
}

// handleError creates an appropriate error from the API response
func (c *CopilotClient) handleError(statusCode int, body []byte) error {
	var errResp AzureErrorResponse
	errMsg := fmt.Sprintf("status code %d", statusCode)
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		errMsg = errResp.Error.Message
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return &APIError{
			StatusCode: statusCode,
			Message:    "Copilot token expired or invalid. Please run 'ai login' again",
		}
	case http.StatusForbidden:
		return &APIError{
			StatusCode: statusCode,
			Message:    "Access denied. Make sure you have an active GitHub Copilot subscription",
		}
	case http.StatusTooManyRequests:
		return &APIError{
			StatusCode: statusCode,
			Message:    "Rate limited. Please wait a moment and try again",
		}
	default:
		return &APIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("Copilot API error: %s", errMsg),
		}
	}
}
