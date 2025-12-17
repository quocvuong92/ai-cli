package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
)

// SSEProcessor handles Server-Sent Events stream processing
type SSEProcessor struct {
	reader         *bufio.Reader
	contentBuilder strings.Builder
	toolCallsMap   map[int]*ToolCall
	finalUsage     Usage
	responseID     string
}

// NewSSEProcessor creates a new SSE stream processor
func NewSSEProcessor(r io.Reader) *SSEProcessor {
	return &SSEProcessor{
		reader:       bufio.NewReader(r),
		toolCallsMap: make(map[int]*ToolCall),
	}
}

// Process reads and processes the SSE stream, calling onChunk for each content chunk
// and returning the final accumulated response when done
func (p *SSEProcessor) Process(ctx context.Context, onChunk func(content string)) error {
	for {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		line, err := p.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
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
			p.responseID = chunk.ID
		}

		// Send content chunk and accumulate
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			p.contentBuilder.WriteString(content)
			onChunk(content)
		}

		// Accumulate tool calls from streaming chunks
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			for _, tc := range chunk.Choices[0].Delta.ToolCalls {
				idx := tc.Index
				if existing, ok := p.toolCallsMap[idx]; ok {
					// Append to existing tool call
					existing.Function.Arguments += tc.Function.Arguments
				} else {
					// Create new tool call entry
					p.toolCallsMap[idx] = &ToolCall{
						ID:    tc.ID,
						Type:  tc.Type,
						Index: idx,
					}
					p.toolCallsMap[idx].Function.Name = tc.Function.Name
					p.toolCallsMap[idx].Function.Arguments = tc.Function.Arguments
				}
			}
		}

		// Capture usage from final chunk
		if chunk.Usage.TotalTokens > 0 {
			p.finalUsage = chunk.Usage
		}
	}

	return nil
}

// BuildResponse constructs the final ChatResponse from accumulated data
func (p *SSEProcessor) BuildResponse() *ChatResponse {
	var toolCalls []ToolCall
	for i := 0; i < len(p.toolCallsMap); i++ {
		if tc, ok := p.toolCallsMap[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &ChatResponse{
		ID: p.responseID,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:      "assistant",
					Content:   p.contentBuilder.String(),
					ToolCalls: toolCalls,
				},
				FinishReason: finishReason,
			},
		},
		Usage: p.finalUsage,
	}
}

// GetContent returns the accumulated content
func (p *SSEProcessor) GetContent() string {
	return p.contentBuilder.String()
}

// HasToolCalls returns true if tool calls were accumulated
func (p *SSEProcessor) HasToolCalls() bool {
	return len(p.toolCallsMap) > 0
}
