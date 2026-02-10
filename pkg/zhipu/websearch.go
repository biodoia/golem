package zhipu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type WebSearchRequest struct {
	SearchEngine        string `json:"search_engine"`
	SearchQuery         string `json:"search_query"`
	Count               int    `json:"count,omitempty"`
	SearchDomainFilter  string `json:"search_domain_filter,omitempty"`
	SearchRecencyFilter string `json:"search_recency_filter,omitempty"`
	RequestID           string `json:"request_id,omitempty"`
	UserID              string `json:"user_id,omitempty"`
}

type WebSearchResponse struct {
	ID           string            `json:"id"`
	Created      int64             `json:"created"`
	SearchResult []WebSearchResult `json:"search_result"`
}

type WebSearchResult struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	Link        string `json:"link"`
	Media       string `json:"media"`
	Icon        string `json:"icon"`
	Refer       string `json:"refer"`
	PublishDate string `json:"publish_date"`
}

func (c *Client) WebSearch(ctx context.Context, req *WebSearchRequest) (*WebSearchResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/web_search", bytes.NewReader(body))
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

	var out WebSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}
