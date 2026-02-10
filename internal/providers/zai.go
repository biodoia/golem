package providers

import (
	"context"

	"github.com/biodoia/golem/pkg/zhipu"
)

type ZAIProvider struct {
	client *zhipu.Client
}

func NewZAIProvider(apiKey string) *ZAIProvider {
	return &ZAIProvider{client: zhipu.NewClient(apiKey)}
}

func (p *ZAIProvider) Chat(ctx context.Context, model string, input string) (*zhipu.ChatResponse, error) {
	return p.client.Chat(ctx, &zhipu.ChatRequest{
		Model:    model,
		Messages: []zhipu.Message{{Role: "user", Content: input}},
	})
}
