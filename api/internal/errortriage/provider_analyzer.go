package errortriage

import (
	"context"
	"log/slog"

	"github.com/xiaoboyu/unipost-api/internal/aiproviders"
)

type ProviderAnalyzer struct {
	providers *aiproviders.Service
	fallback  Analyzer
}

func NewProviderAnalyzer(providers *aiproviders.Service, fallback Analyzer) *ProviderAnalyzer {
	if fallback == nil {
		fallback = DeterministicAnalyzer{}
	}
	return &ProviderAnalyzer{providers: providers, fallback: fallback}
}

func (a *ProviderAnalyzer) Analyze(bucket Bucket) ItemDraft {
	analyzer, _ := a.AnalyzerForRun(context.Background())
	return analyzer.Analyze(bucket)
}

func (a *ProviderAnalyzer) AnalyzerForRun(ctx context.Context) (Analyzer, string) {
	if a == nil || a.providers == nil {
		return a.fallback, "deterministic"
	}
	cfg, err := a.providers.Resolve(ctx, aiproviders.SurfaceErrorTriage)
	if err != nil {
		slog.Warn("error triage: AI provider unavailable, using deterministic analyzer", "error", err)
		return a.fallback, "deterministic"
	}
	if cfg.ClientKind != aiproviders.ClientKindChatCompletions || cfg.APIKey == "" {
		return a.fallback, "deterministic"
	}
	analyzer := NewOpenAIAnalyzer(cfg.APIKey, cfg.Model, cfg.ChatCompletionsURL(), nil, a.fallback)
	return analyzer, cfg.ModelName()
}
