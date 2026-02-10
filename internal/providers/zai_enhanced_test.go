package providers

import (
	"context"
	"testing"
	"time"
)

func TestEnhancedZAIProvider_RegisterTool(t *testing.T) {
	provider := NewEnhancedZAIProvider("test-key")

	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters:  map[string]interface{}{},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "test result", nil
		},
	}

	provider.RegisterTool(tool)

	registry := provider.GetToolRegistry()
	if len(registry) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(registry))
	}

	if _, ok := registry["test_tool"]; !ok {
		t.Error("Tool not found in registry")
	}
}

func TestEnhancedZAIProvider_StreamEvents(t *testing.T) {
	provider := NewEnhancedZAIProvider("test-key")

	// Register a simple tool
	provider.RegisterTool(Tool{
		Name:        "echo",
		Description: "Echoes back the input",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"message"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			msg, _ := args["message"].(string)
			return "Echo: " + msg, nil
		},
	})

	// Test stream events
	var events []StreamEvent
	handler := func(event StreamEvent) {
		events = append(events, event)
	}

	// Simulate streaming (without actual API call)
	provider.streamBuffer.WriteString("Hello")
	handler(StreamEvent{
		Type:    EventText,
		Content: "Hello",
	})

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].Type != EventText {
		t.Errorf("Expected EventText, got %s", events[0].Type)
	}
}

func TestDefaultTools(t *testing.T) {
	tools := DefaultTools()

	expectedTools := []string{"shell", "read_file", "write_file"}
	
	for _, name := range expectedTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("Expected tool %s not found", name)
		}
	}

	// Test shell tool
	shell := tools["shell"]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := shell.Handler(ctx, map[string]interface{}{"command": "ls"})
	if err != nil {
		t.Errorf("Shell tool error: %v", err)
	}

	expected := "Would execute: ls"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestBuildToolSystemPrompt(t *testing.T) {
	provider := NewEnhancedZAIProvider("test-key")

	// Register tools
	provider.RegisterTool(Tool{
		Name:        "tool1",
		Description: "First tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "First parameter",
				},
			},
			"required": []string{"param1"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "", nil
		},
	})

	prompt := provider.buildToolSystemPrompt()

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	// Check that tool is mentioned
	if !contains(prompt, "tool1") {
		t.Error("Tool not mentioned in prompt")
	}

	// Check that parameter is mentioned
	if !contains(prompt, "param1") {
		t.Error("Parameter not mentioned in prompt")
	}
}

func TestClearTools(t *testing.T) {
	provider := NewEnhancedZAIProvider("test-key")

	// Add a tool
	provider.RegisterTool(Tool{
		Name: "test",
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "", nil
		},
	})

	if len(provider.GetToolRegistry()) != 1 {
		t.Error("Tool not registered")
	}

	// Clear tools
	provider.ClearTools()

	if len(provider.GetToolRegistry()) != 0 {
		t.Error("Tools not cleared")
	}
}

func TestGetBufferedContent(t *testing.T) {
	provider := NewEnhancedZAIProvider("test-key")

	// Write to buffer
	provider.streamBuffer.WriteString("test content")

	content := provider.GetBufferedContent()

	if content != "test content" {
		t.Errorf("Expected %q, got %q", "test content", content)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}