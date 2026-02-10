package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/biodoia/golem/internal/config"
	"github.com/biodoia/golem/pkg/zhipu"
)

func RunOneShot(query string) error {
	return RunOneShotStream(query)
}

// RunOneShot executes a query with streaming output
func RunOneShotStream(query string) error {
	settings, err := config.Load()
	if err != nil {
		return err
	}
	if settings.APIKey == "" {
		return fmt.Errorf("missing API key. Set ZAI_API_KEY or ZHIPU_API_KEY")
	}
	client := zhipu.NewClient(settings.APIKey)

	ctx := context.Background()
	textCh, errCh := client.ChatStream(ctx, &zhipu.ChatRequest{
		Model:    settings.Model,
		Messages: []zhipu.Message{{Role: "user", Content: query}},
	})

	for {
		select {
		case text, ok := <-textCh:
			if !ok {
				fmt.Println() // End with newline
				return nil
			}
			if text != "" {
				fmt.Print(text)
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				return err
			}
			fmt.Println()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// RunOneShotTools executes a query with function calling support
func RunOneShotTools(query string, tools []zhipu.Tool, toolHandlers map[string]func(map[string]interface{}) (string, error)) error {
	settings, err := config.Load()
	if err != nil {
		return err
	}
	if settings.APIKey == "" {
		return fmt.Errorf("missing API key. Set ZAI_API_KEY or ZHIPU_API_KEY")
	}
	client := zhipu.NewClient(settings.APIKey)

	messages := []zhipu.Message{{Role: "user", Content: query}}
	maxToolCalls := 5 // Prevent infinite loops
	toolCallCount := 0

	ctx := context.Background()

	for {
		resp, err := client.Chat(ctx, &zhipu.ChatRequest{
			Model:      settings.Model,
			Messages:   messages,
			Tools:      tools,
			ToolChoice: "auto",
		})
		if err != nil {
			return err
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("empty response")
		}

		choice := resp.Choices[0]
		content, ok := choice.Message.Content.(string)
		if ok && content != "" {
			fmt.Print(content)
		}

		// Check for tool calls
		if len(choice.Message.ToolCalls) == 0 {
			fmt.Println()
			return nil
		}

		toolCallCount++
		if toolCallCount > maxToolCalls {
			return fmt.Errorf("max tool calls (%d) exceeded", maxToolCalls)
		}

		// Execute tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			fmt.Fprintf(os.Stderr, "\n[Tool: %s]\n", toolCall.Function.Name)

			// Find and execute handler
			handler, ok := toolHandlers[toolCall.Function.Name]
			if !ok {
				result := fmt.Sprintf("Error: tool not found: %s", toolCall.Function.Name)
				messages = append(messages, zhipu.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			var args map[string]interface{}
			if err := parseJSONArgs(toolCall.Function.Arguments, &args); err != nil {
				result := fmt.Sprintf("Error: invalid arguments: %v", err)
				messages = append(messages, zhipu.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			result, err := handler(args)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}
			messages = append(messages, zhipu.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}

func parseJSONArgs(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}
