package zhipu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-api-key")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.apiKey != "test-api-key" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "test-api-key")
	}
	if c.baseURL != BaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, BaseURL)
	}
}

func TestAllModels(t *testing.T) {
	models := AllModels()
	if len(models) == 0 {
		t.Fatal("AllModels returned empty slice")
	}

	expectedModels := []string{
		ModelGLM4_32B,
		ModelGLM4_9B,
		ModelGLMZ1_32B,
		ModelCodeGeeX4,
	}

	for _, expected := range expectedModels {
		found := false
		for _, m := range models {
			if m == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected model %q not found in AllModels()", expected)
		}
	}
}

func TestChatRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", auth)
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Model != ModelGLM4_32B {
			t.Errorf("expected model %s, got %s", ModelGLM4_32B, req.Model)
		}

		resp := ChatResponse{
			ID:      "test-id",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []struct {
				Index        int     `json:"index"`
				Message      Message `json:"message"`
				FinishReason string  `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "Hello, I'm GLM!",
					},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.baseURL = server.URL

	resp, err := c.Chat(context.Background(), &ChatRequest{
		Model: ModelGLM4_32B,
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.ID != "test-id" {
		t.Errorf("response ID = %q, want %q", resp.ID, "test-id")
	}
	if len(resp.Choices) == 0 {
		t.Fatal("no choices in response")
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	if content != "Hello, I'm GLM!" {
		t.Errorf("content = %q, want %q", content, "Hello, I'm GLM!")
	}
}

func TestChatStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`data: {"id":"1","choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"id":"1","choices":[{"delta":{"content":", "}}]}`,
			`data: {"id":"1","choices":[{"delta":{"content":"world!"}}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			w.Write([]byte(event + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.baseURL = server.URL

	textCh, errCh := c.ChatStream(context.Background(), &ChatRequest{
		Model: ModelGLM4_32B,
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
	})

	var content strings.Builder
	for {
		select {
		case text, ok := <-textCh:
			if !ok {
				goto done
			}
			content.WriteString(text)
		case err, ok := <-errCh:
			if ok && err != nil {
				t.Fatalf("stream error: %v", err)
			}
		}
	}
done:
	if content.String() != "Hello, world!" {
		t.Errorf("streamed content = %q, want %q", content.String(), "Hello, world!")
	}
}

func TestToolDefinition(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: &Function{
			Name:        "get_weather",
			Description: "Get the current weather",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "City name",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("failed to marshal tool: %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal tool: %v", err)
	}

	if decoded.Type != "function" {
		t.Errorf("type = %q, want %q", decoded.Type, "function")
	}
	if decoded.Function.Name != "get_weather" {
		t.Errorf("name = %q, want %q", decoded.Function.Name, "get_weather")
	}
}

func TestStreamResult(t *testing.T) {
	textCh := make(chan string, 3)
	toolCh := make(chan ToolCall, 1)
	errCh := make(chan error, 1)

	textCh <- "Hello"
	textCh <- ", "
	textCh <- "world!"
	close(textCh)

	toolCh <- ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{
			Name:      "test_func",
			Arguments: `{"arg": "value"}`,
		},
	}
	close(toolCh)
	close(errCh)

	result := CollectStream(textCh, toolCh, errCh)

	if result.Content != "Hello, world!" {
		t.Errorf("content = %q, want %q", result.Content, "Hello, world!")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "test_func" {
		t.Errorf("tool call name = %q, want %q", result.ToolCalls[0].Function.Name, "test_func")
	}
	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
}

func TestMessage(t *testing.T) {
	// Test simple text message
	msg := Message{
		Role:    "user",
		Content: "Hello",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if decoded.Role != "user" {
		t.Errorf("role = %q, want %q", decoded.Role, "user")
	}
	content, _ := decoded.Content.(string)
	if content != "Hello" {
		t.Errorf("content = %q, want %q", content, "Hello")
	}
}

func TestMultimodalContent(t *testing.T) {
	parts := []ContentPart{
		{Type: "text", Text: "What's in this image?"},
		{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/image.jpg"}},
	}

	msg := Message{
		Role:    "user",
		Content: parts,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal multimodal message: %v", err)
	}

	if !strings.Contains(string(data), "image_url") {
		t.Error("expected image_url in marshaled data")
	}
}
