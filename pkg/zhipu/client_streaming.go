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

// ChatStreamWithTools streams chat completion with tool call support
// Returns separate channels for text chunks, tool calls, and errors
func (c *Client) ChatStreamWithTools(ctx context.Context, req *ChatRequest) (<-chan string, <-chan ToolCall, <-chan error) {
	req.Stream = true

	textCh := make(chan string, 100)
	toolCh := make(chan ToolCall, 10)
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

		// Accumulate tool call fragments (streamed incrementally)
		toolCallBuilders := make(map[int]*toolCallBuilder)

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
				// Emit any completed tool calls
				for _, b := range toolCallBuilders {
					if b.complete() {
						toolCh <- b.build()
					}
				}
				return
			}

			var event streamEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				continue
			}

			for _, choice := range event.Choices {
				// Handle text content
				if choice.Delta.Content != "" {
					textCh <- choice.Delta.Content
				}

				// Handle tool calls (streamed as deltas)
				for _, tcDelta := range choice.Delta.ToolCalls {
					idx := tcDelta.Index
					if _, exists := toolCallBuilders[idx]; !exists {
						toolCallBuilders[idx] = &toolCallBuilder{}
					}
					b := toolCallBuilders[idx]

					if tcDelta.ID != "" {
						b.id = tcDelta.ID
					}
					if tcDelta.Type != "" {
						b.typ = tcDelta.Type
					}
					if tcDelta.Function.Name != "" {
						b.funcName = tcDelta.Function.Name
					}
					if tcDelta.Function.Arguments != "" {
						b.funcArgs += tcDelta.Function.Arguments
					}
				}

				// Check if finish_reason indicates completion
				if choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" {
					for _, b := range toolCallBuilders {
						if b.complete() {
							toolCh <- b.build()
						}
					}
					toolCallBuilders = make(map[int]*toolCallBuilder)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return textCh, toolCh, errCh
}

// streamEvent represents a streaming SSE event
type streamEvent struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string          `json:"role,omitempty"`
			Content   string          `json:"content"`
			ToolCalls []toolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// toolCallDelta represents incremental tool call data in stream
type toolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// toolCallBuilder accumulates streamed tool call fragments
type toolCallBuilder struct {
	id       string
	typ      string
	funcName string
	funcArgs string
}

func (b *toolCallBuilder) complete() bool {
	return b.id != "" && b.funcName != ""
}

func (b *toolCallBuilder) build() ToolCall {
	return ToolCall{
		ID:   b.id,
		Type: b.typ,
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{
			Name:      b.funcName,
			Arguments: b.funcArgs,
		},
	}
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
