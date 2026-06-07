package reviewai

import (
	"context"
	"fmt"

	"github.com/xiaoboyu/unipost-api/internal/aiproviders"
)

type ProviderPlanner struct {
	providers *aiproviders.Service
}

func NewProviderPlanner(providers *aiproviders.Service) *ProviderPlanner {
	return &ProviderPlanner{providers: providers}
}

func (p *ProviderPlanner) NextAction(ctx context.Context, obs Observation, goal string) (Action, error) {
	if p == nil || p.providers == nil {
		return Action{}, fmt.Errorf("ANTHROPIC_API_KEY not configured")
	}
	cfg, err := p.providers.Resolve(ctx, aiproviders.SurfaceAppReviewAI)
	if err != nil {
		return Action{}, err
	}
	if cfg.ClientKind != aiproviders.ClientKindMessages {
		return Action{}, fmt.Errorf("AI provider is not configured for messages")
	}
	var client *AnthropicClient
	if cfg.Provider == aiproviders.ProviderAnthropic {
		client = NewAnthropicClient(cfg.APIKey, cfg.Model, cfg.MessagesURL(), nil)
	} else {
		client = NewTokenGateMessagesClient(cfg.APIKey, cfg.Model, cfg.MessagesURL(), nil)
	}
	return client.NextAction(ctx, obs, goal)
}
