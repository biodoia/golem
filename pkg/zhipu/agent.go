package zhipu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type AgentChatRequest struct {
	AgentID         string      `json:"agent_id"`
	Stream          bool        `json:"stream,omitempty"`
	Messages        []Message   `json:"messages"`
	CustomVariables interface{} `json:"custom_variables,omitempty"`
}

type AgentChatResponse struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Messages     []struct {
			Role    string `json:"role"`
			Content struct {
				Text string `json:"text"`
				Type string `json:"type"`
			} `json:"content"`
		} `json:"messages"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		TotalCalls       int `json:"total_calls"`
	} `json:"usage"`
}

func (c *Client) AgentChat(ctx context.Context, req *AgentChatRequest) (*AgentChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.z.ai/api/v1/agents", bytes.NewReader(body))
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
	var out AgentChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}
