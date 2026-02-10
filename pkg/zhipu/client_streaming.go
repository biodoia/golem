// Package zhipu provides native Z.AI / Zhipu AI API client
// This file adds enhanced streaming support with tool call handling
package zhipu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// StreamEvent represents a parsed streaming event
type StreamEvent struct {
	ID      string       `json:"id"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a choice in streaming response
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// StreamDelta represents incremental content in streaming
type StreamDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatStreamWithTools sends a streaming chat completion with tool support
// Returns channels for text content, tool calls, and errors
func (c *Client) ChatStreamWithTools(ctx context.Context, req *ChatRequest) (<-chan string, <-chan ToolCall, <-chan error) {
	req.Stream = true

	textCh := make(chan string)
	toolCh := make(chan ToolCall)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
		defer close(toolCh)
		defer close(errCh)

		body, err := json.Marshal(req)
		if err != nil {
			errCh <- fmt.Errorf("marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			errCh <- fmt.Errorf("create request: %w", err)
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("do request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errCh <- parseAPIError(resp)
			return
		}

		// Track partial tool calls being built up
		toolCallBuffer := make(map[int]*ToolCall) // index -> partial tool call

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				// Flush any remaining tool calls
				for _, tc := range toolCallBuffer {
					if tc != nil && tc.ID != "" {
						toolCh <- *tc
					}
				}
				return
			}

			var event StreamEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				continue
			}

			for _, choice := range event.Choices {
				// Handle text content
				if choice.Delta.Content != "" {
					textCh <- choice.Delta.Content
				}

				// Handle tool calls (may come in chunks)
				for _, tc := range choice.Delta.ToolCalls {
					idx := 0 // Default index
					if tc.ID != "" {
						// New tool call starting
						existing := toolCallBuffer[idx]
						if existing != nil && existing.ID != "" {
							// Flush the previous tool call
							toolCh <- *existing
						}
						toolCallBuffer[idx] = &ToolCall{
							ID:   tc.ID,
							Type: tc.Type,
							Function: struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							}{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						}
					} else if toolCallBuffer[idx] != nil {
						// Continuation of existing tool call (arguments chunk)
						toolCallBuffer[idx].Function.Arguments += tc.Function.Arguments
					}
				}

				// Check if finish_reason indicates tool_calls complete
				if choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" {
					for _, tc := range toolCallBuffer {
						if tc != nil && tc.ID != "" {
							toolCh <- *tc
						}
					}
					toolCallBuffer = make(map[int]*ToolCall)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return textCh, toolCh, errCh
}

// StreamResult collects all streaming output into a final result
type StreamResult struct {
	Content   string
	ToolCalls []ToolCall
	Error     error
}

// CollectStream collects all streaming output synchronously
func CollectStream(textCh <-chan string, toolCh <-chan ToolCall, errCh <-chan error) StreamResult {
	var result StreamResult
	var contentBuilder strings.Builder

	for {
		select {
		case text, ok := <-textCh:
			if !ok {
				textCh = nil
			} else {
				contentBuilder.WriteString(text)
			}
		case tc, ok := <-toolCh:
			if !ok {
				toolCh = nil
			} else {
				result.ToolCalls = append(result.ToolCalls, tc)
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				result.Error = err
			}
			errCh = nil
		}

		if textCh == nil && toolCh == nil && errCh == nil {
			break
		}
	}

	result.Content = contentBuilder.String()
	return result
}
