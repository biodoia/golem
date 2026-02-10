// Package zhipu provides native Z.AI / Zhipu AI API client
package zhipu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	BaseURL = "https://api.z.ai/api/paas/v4"

	// Models
	ModelGLM4_32B        = "glm-4-32b-0414"
	ModelGLM4_9B         = "glm-4-9b-chat"
	ModelGLMZ1_32B       = "glm-z1-32b-0414"
	ModelGLMZ1Rumination = "glm-z1-rumination-32b-0414"
	ModelGLMZ1_9B        = "glm-z1-9b-0414"
	ModelGLM4V           = "glm-4v"
	ModelGLM4VPlus       = "glm-4v-plus"
	ModelGLM4VThinking   = "glm-4.1v-9b-thinking"
	ModelCodeGeeX4       = "codegeex-4"

	// Embedding
	ModelEmbedding3 = "embedding-3"
)

// Client is the Zhipu AI API client
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Zhipu AI client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL: BaseURL,
	}
}

// Message represents a chat message
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string or []ContentPart
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentPart for multimodal content
type ContentPart struct {
	Type     string    `json:"type"` // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL for vision models
type ImageURL struct {
	URL string `json:"url"`
}

// Tool definition for function calling
type Tool struct {
	Type      string     `json:"type"` // "function" or "web_search" or "retrieval" or "code_interpreter"
	Function  *Function  `json:"function,omitempty"`
	WebSearch *WebSearch `json:"web_search,omitempty"`
	Retrieval *Retrieval `json:"retrieval,omitempty"`
}

// Function definition
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// WebSearch tool config
type WebSearch struct {
	Enable      bool   `json:"enable"`
	SearchQuery string `json:"search_query,omitempty"`
}

// Retrieval tool config (RAG)
type Retrieval struct {
	KnowledgeID    string `json:"knowledge_id"`
	PromptTemplate string `json:"prompt_template,omitempty"`
}

// ToolCall in response
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ChatRequest for chat completions
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"` // "auto" or "none"

	// Z1 specific
	DoSample bool `json:"do_sample,omitempty"`

	// Rumination specific
	SearchMode string `json:"search_mode,omitempty"` // "search_std" or "search_pro"
}

// ChatResponse from chat completions
type ChatResponse struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`

	// Rumination specific
	WebSearch []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Content string `json:"content"`
	} `json:"web_search,omitempty"`
}

// Chat sends a chat completion request
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatStream sends a streaming chat completion request
func (c *Client) ChatStream(ctx context.Context, req *ChatRequest) (<-chan string, <-chan error) {
	req.Stream = true

	textCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
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

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				return
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				continue
			}
			if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
				choice, _ := choices[0].(map[string]interface{})
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok && content != "" {
						textCh <- content
					}
				}
				if message, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok && content != "" {
						textCh <- content
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return textCh, errCh
}

// Embedding generates embeddings
func (c *Client) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	if model == "" {
		model = ModelEmbedding3
	}

	reqBody := map[string]interface{}{
		"model": model,
		"input": input,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	var embResp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data")
	}

	return embResp.Data[0].Embedding, nil
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var payload struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Message != "" {
		return fmt.Errorf("API error %d/%d: %s", resp.StatusCode, payload.Error.Code, payload.Error.Message)
	}
	return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
}

// CodeGeeX specific methods
func (c *Client) CodeCompletion(ctx context.Context, prompt string, language string) (string, error) {
	req := &ChatRequest{
		Model: ModelCodeGeeX4,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := c.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion")
	}

	content, _ := resp.Choices[0].Message.Content.(string)
	return content, nil
}

// AllModels returns all available models
func AllModels() []string {
	return []string{
		ModelGLM4_32B,
		ModelGLM4_9B,
		ModelGLMZ1_32B,
		ModelGLMZ1Rumination,
		ModelGLMZ1_9B,
		ModelGLM4V,
		ModelGLM4VPlus,
		ModelGLM4VThinking,
		ModelCodeGeeX4,
		ModelEmbedding3,
	}
}
