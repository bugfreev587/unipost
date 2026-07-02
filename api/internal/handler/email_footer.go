package handler

import (
	"context"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
)

func emailFooterVariables(ctx context.Context, eventKey, userID, email, appBaseURL string, vars map[string]any) map[string]any {
	decision, err := emailpolicy.NewService(nil, appBaseURL).Prepare(ctx, emailpolicy.Request{
		EventKey:      eventKey,
		UserID:        userID,
		Email:         email,
		DataVariables: vars,
	})
	if err != nil {
		return vars
	}
	return decision.DataVariables
}
