package worker

import (
	"context"
	"log/slog"

	"github.com/xiaoboyu/unipost-api/internal/platformcontrols"
)

func platformReadBlocked(ctx context.Context, evaluator platformcontrols.Evaluator, platformName, component string) bool {
	if evaluator == nil {
		return false
	}
	decision, err := evaluator.Evaluate(ctx, platformName, platformcontrols.OperationRead)
	if err != nil {
		slog.Error("platform operational control unavailable; skipping provider call", "component", component, "platform", platformName, "error", err)
		return true
	}
	if decision.Blocked {
		slog.Warn("platform provider call blocked by operational control", "component", component, "platform", platformName, "state", decision.State)
		return true
	}
	return false
}
