// Package providers implements enhanced AI model providers for GOLEM
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/biodoia/golem/pkg/zhipu"
)

// EnhancedZAIProvider extends ZAIProvider with advanced streaming and tool calling
type EnhancedZAIProvider struct {
	*ZAIProvider
	toolRegistry map[string]Tool
	streamBuffer strings.Builder
}

// Tool represents a callable function
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Handler     ToolHandler
}

// ToolHandler executes a tool
type ToolHandler func(ctx context.Context, args map[string]interface{}) (string, error)

// StreamEvent represents a streaming event
type StreamEvent struct {
	Type      StreamEventType `json:"type"`
	Content   string         `json:"content,omitempty"`
	ToolCall  *ToolCallEvent `json:"tool_call,omitempty"`
	Error     string         `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StreamEventType defines the type of streaming event
type StreamEventType string

const (
	EventText       StreamEventType = "text"
	EventToolCall   StreamEventType = "tool_call"
	EventToolResult StreamEventType = "tool_result"
	EventError      StreamEventType = "error"
	EventDone       StreamEventType = "done"
)

// ToolCallEvent represents a tool call in the stream
type ToolCallEvent struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// StreamHandler handles streaming events
type StreamHandler func(event StreamEvent)

// NewEnhancedZAIProvider creates an enhanced provider
func NewEnhancedZAIProvider(apiKey string) *EnhancedZAIProvider {
	base := NewZAIProvider(apiKey)
	return &EnhancedZAIProvider{
		ZAIProvider:  base,
		toolRegistry: make(map[string]Tool),
	}
}

// RegisterTool registers a tool for function calling
func (p *EnhancedZAIProvider) RegisterTool(tool Tool) {
	p.toolRegistry[tool.Name] = tool
}

// ChatStreamWithTools sends a streaming request with tool support
func (p *EnhancedZAIProvider) ChatStreamWithTools(ctx context.Context, input string, handler StreamHandler) error {
	// Convert tools to zhipu format
	zhipuTools := make([]zhipu.Tool, 0, len(p.toolRegistry))
	for _, tool := range p.toolRegistry {
		zhipuTools = append(zhipuTools, zhipu.Tool{
			Type: "function",
			Function: &zhipu.Function{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	p.RegisterTools(zhipuTools)

	// Start streaming with tool support
	return p.chatStreamInternal(ctx, input, handler)
}

// chatStreamInternal handles the internal streaming logic
func (p *EnhancedZAIProvider) chatStreamInternal(ctx context.Context, input string, handler StreamHandler) error {
	// Clear buffer
	p.streamBuffer.Reset()

	// Add system message about available tools
	if len(p.toolRegistry) > 0 {
		p.AddSystemMessage(p.buildToolSystemPrompt())
	}

	messages := append(p.history, zhipu.Message{Role: "user", Content: input})

	// Start the chat stream
	textCh, errCh := p.client.ChatStream(ctx, &zhipu.ChatRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: p.temperature,
		Stream:      true,
		Tools:       p.tools,
		ToolChoice:  "auto",
	})

	// Collect streaming text
	var textBuilder strings.Builder
	var currentToolCall *zhipu.ToolCall

	for {
		select {
		case text, ok := <-textCh:
			if !ok {
				// Stream ended - process any pending tool calls
				if currentToolCall != nil {
					if err := p.executeToolCall(ctx, currentToolCall, handler); err != nil {
						handler(StreamEvent{
							Type:  EventError,
							Error: err.Error(),
						})
						return err
					}
				}

				// Save to history
				fullResponse := textBuilder.String()
				if fullResponse != "" {
					p.history = append(messages, zhipu.Message{Role: "assistant", Content: fullResponse})
				}

				// Send done event
				handler(StreamEvent{
					Type: EventDone,
					Metadata: map[string]interface{}{
						"tokens_used": len(strings.Split(fullResponse, " ")),
					},
				})
				return nil
			}

			// Send text event
			handler(StreamEvent{
				Type:    EventText,
				Content: text,
			})

			textBuilder.WriteString(text)
			p.streamBuffer.WriteString(text)

		case err := <-errCh:
			if err != nil {
				handler(StreamEvent{
					Type:  EventError,
					Error: err.Error(),
				})
				return err
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// executeToolCall executes a tool call and streams the result
func (p *EnhancedZAIProvider) executeToolCall(ctx context.Context, toolCall *zhipu.ToolCall, handler StreamHandler) error {
	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}

	// Send tool call event
	handler(StreamEvent{
		Type: EventToolCall,
		ToolCall: &ToolCallEvent{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: args,
		},
	})

	// Execute tool
	tool, ok := p.toolRegistry[toolCall.Function.Name]
	if !ok {
		return fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Execute handler
	result, err := tool.Handler(ctx, args)
	if err != nil {
		result = fmt.Sprintf("Error: %v", err)
	}

	// Send tool result event
	handler(StreamEvent{
		Type:    EventToolResult,
		Content: result,
		Metadata: map[string]interface{}{
			"tool_name": toolCall.Function.Name,
		},
	})

	return nil
}

// buildToolSystemPrompt creates a system prompt describing available tools
func (p *EnhancedZAIProvider) buildToolSystemPrompt() string {
	if len(p.toolRegistry) == 0 {
		return ""
	}

	var prompt strings.Builder
	prompt.WriteString("You have access to the following tools:\n\n")

	for _, tool := range p.toolRegistry {
		prompt.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
		if params, ok := tool.Parameters["properties"].(map[string]interface{}); ok {
			prompt.WriteString("  Parameters:\n")
			for name, param := range params {
				if paramInfo, ok := param.(map[string]interface{}); ok {
					desc, _ := paramInfo["description"].(string)
					paramType, _ := paramInfo["type"].(string)
					prompt.WriteString(fmt.Sprintf("    - %s (%s): %s\n", name, paramType, desc))
				}
			}
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString("To use a tool, output a JSON object with the tool call.\n")
	prompt.WriteString("Example: {\"tool\": \"function_name\", \"arguments\": {...}}\n")

	return prompt.String()
}

// GetToolRegistry returns all registered tools
func (p *EnhancedZAIProvider) GetToolRegistry() map[string]Tool {
	registry := make(map[string]Tool)
	for k, v := range p.toolRegistry {
		registry[k] = v
	}
	return registry
}

// ClearTools clears all registered tools
func (p *EnhancedZAIProvider) ClearTools() {
	p.toolRegistry = make(map[string]Tool)
}

// GetBufferedContent returns all buffered content
func (p *EnhancedZAIProvider) GetBufferedContent() string {
	return p.streamBuffer.String()
}

// DefaultTools returns a set of common tools
func DefaultTools() map[string]Tool {
	return map[string]Tool{
		"shell": {
			Name:        "shell",
			Description: "Execute shell commands",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command to execute",
					},
				},
				"required": []string{"command"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
				cmd, ok := args["command"].(string)
				if !ok {
					return "", fmt.Errorf("command is required")
				}
				
				// Execute command (in production, use proper sandbox)
				// For now, return a placeholder
				return fmt.Sprintf("Would execute: %s", cmd), nil
			},
		},

		"read_file": {
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
				},
				"required": []string{"path"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
				path, ok := args["path"].(string)
				if !ok {
					return "", fmt.Errorf("path is required")
				}
				
				// Read file (implementation depends on file system access)
				return fmt.Sprintf("Would read file: %s", path), nil
			},
		},

		"write_file": {
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write",
					},
				},
				"required": []string{"path", "content"},
			},
			Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
				path, _ := args["path"].(string)
				content, _ := args["content"].(string)
				
				// Write file (implementation depends on file system access)
				return fmt.Sprintf("Would write %d bytes to: %s", len(content), path), nil
			},
		},
	}
}