// Package providers implements AI model providers for GOLEM
package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/biodoia/golem/pkg/zhipu"
)

// StreamCallback receives streaming text chunks
type StreamCallback func(text string)

// ToolCall represents a function call from the model
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

// ZAIProvider wraps the Z.AI API client with enhanced features
type ZAIProvider struct {
	client      *zhipu.Client
	model       string
	temperature float64
	tools       []zhipu.Tool
	history     []zhipu.Message
}

// NewZAIProvider creates a new Z.AI provider
func NewZAIProvider(apiKey string) *ZAIProvider {
	return &ZAIProvider{
		client:      zhipu.NewClient(apiKey),
		model:       zhipu.ModelGLM4_32B,
		temperature: 0.7,
		history:     make([]zhipu.Message, 0),
	}
}

// SetModel changes the active model
func (p *ZAIProvider) SetModel(model string) {
	p.model = model
}

// SetTemperature adjusts creativity
func (p *ZAIProvider) SetTemperature(temp float64) {
	p.temperature = temp
}

// RegisterTools registers function calling tools
func (p *ZAIProvider) RegisterTools(tools []zhipu.Tool) {
	p.tools = tools
}

// ClearHistory resets conversation history
func (p *ZAIProvider) ClearHistory() {
	p.history = make([]zhipu.Message, 0)
}

// AddSystemMessage adds a system prompt
func (p *ZAIProvider) AddSystemMessage(content string) {
	p.history = append([]zhipu.Message{{Role: "system", Content: content}}, p.history...)
}

// Chat sends a non-streaming request
func (p *ZAIProvider) Chat(ctx context.Context, model string, input string) (*zhipu.ChatResponse, error) {
	if model == "" {
		model = p.model
	}

	messages := append(p.history, zhipu.Message{Role: "user", Content: input})

	req := &zhipu.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: p.temperature,
	}

	if len(p.tools) > 0 {
		req.Tools = p.tools
		req.ToolChoice = "auto"
	}

	resp, err := p.client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	// Update history
	if len(resp.Choices) > 0 {
		p.history = append(messages, resp.Choices[0].Message)
	}

	return resp, nil
}

// ChatStream sends a streaming request with callback
func (p *ZAIProvider) ChatStream(ctx context.Context, input string, callback StreamCallback) error {
	messages := append(p.history, zhipu.Message{Role: "user", Content: input})

	req := &zhipu.ChatRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: p.temperature,
		Stream:      true,
	}

	textCh, errCh := p.client.ChatStream(ctx, req)

	var fullResponse string
	for {
		select {
		case text, ok := <-textCh:
			if !ok {
				// Stream ended
				if fullResponse != "" {
					p.history = append(messages, zhipu.Message{Role: "assistant", Content: fullResponse})
				}
				return nil
			}
			fullResponse += text
			if callback != nil {
				callback(text)
			}
		case err := <-errCh:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ChatWithTools sends a request and handles tool calls
func (p *ZAIProvider) ChatWithTools(ctx context.Context, input string, executor ToolExecutor) (*zhipu.ChatResponse, error) {
	messages := append(p.history, zhipu.Message{Role: "user", Content: input})

	for {
		req := &zhipu.ChatRequest{
			Model:       p.model,
			Messages:    messages,
			Temperature: p.temperature,
			Tools:       p.tools,
			ToolChoice:  "auto",
		}

		resp, err := p.client.Chat(ctx, req)
		if err != nil {
			return nil, err
		}

		if len(resp.Choices) == 0 {
			return resp, nil
		}

		choice := resp.Choices[0]
		messages = append(messages, choice.Message)

		// Check for tool calls
		if len(choice.Message.ToolCalls) == 0 {
			p.history = messages
			return resp, nil
		}

		// Execute tool calls
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = make(map[string]interface{})
			}

			result, err := executor.Execute(ctx, tc.Function.Name, args)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			messages = append(messages, zhipu.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}
}

// ToolExecutor interface for executing tool calls
type ToolExecutor interface {
	Execute(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// AvailableModels returns all Z.AI models
func AvailableModels() []string {
	return zhipu.AllModels()
}

// ModelInfo describes a model's capabilities
type ModelInfo struct {
	ID          string
	Name        string
	Description string
	Capabilities []string
	Context     int
}

// GetModelInfo returns detailed model information
func GetModelInfo(modelID string) *ModelInfo {
	models := map[string]*ModelInfo{
		zhipu.ModelGLM4_32B: {
			ID:          zhipu.ModelGLM4_32B,
			Name:        "GLM-4-32B",
			Description: "Dialogue, code generation, function calling",
			Capabilities: []string{"chat", "code", "function_calling"},
			Context:     128000,
		},
		zhipu.ModelGLMZ1_32B: {
			ID:          zhipu.ModelGLMZ1_32B,
			Name:        "GLM-Z1-32B",
			Description: "Deep thinking, math, complex reasoning",
			Capabilities: []string{"reasoning", "math", "analysis"},
			Context:     128000,
		},
		zhipu.ModelGLMZ1Rumination: {
			ID:          zhipu.ModelGLMZ1Rumination,
			Name:        "GLM-Z1-Rumination",
			Description: "Research mode with web search augmentation",
			Capabilities: []string{"reasoning", "web_search", "research"},
			Context:     128000,
		},
		zhipu.ModelGLM4V: {
			ID:          zhipu.ModelGLM4V,
			Name:        "GLM-4V",
			Description: "Vision model for image understanding",
			Capabilities: []string{"vision", "image_analysis"},
			Context:     8192,
		},
		zhipu.ModelCodeGeeX4: {
			ID:          zhipu.ModelCodeGeeX4,
			Name:        "CodeGeeX-4",
			Description: "Specialized code completion and generation",
			Capabilities: []string{"code", "completion"},
			Context:     32000,
		},
	}

	if info, ok := models[modelID]; ok {
		return info
	}
	return nil
}
