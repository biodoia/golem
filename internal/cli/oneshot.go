package cli

import (
	"context"
	"fmt"

	"github.com/biodoia/golem/internal/config"
	"github.com/biodoia/golem/pkg/zhipu"
)

func RunOneShot(query string) error {
	settings, err := config.Load()
	if err != nil {
		return err
	}
	if settings.APIKey == "" {
		return fmt.Errorf("missing API key. Set ZAI_API_KEY or ZHIPU_API_KEY")
	}
	client := zhipu.NewClient(settings.APIKey)
	resp, err := client.Chat(context.Background(), &zhipu.ChatRequest{
		Model:    settings.Model,
		Messages: []zhipu.Message{{Role: "user", Content: query}},
	})
	if err != nil {
		return err
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("empty response")
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	fmt.Println(content)
	return nil
}
