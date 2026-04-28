package ratelimit

import (
	"context"
	"testing"
)

func TestNoopLimiter_AllowsEverything(t *testing.T) {
	ctx := context.Background()
	var l Limiter = NoopLimiter{}

	if d, err := l.AllowRequest(ctx, RequestScope{WorkspaceID: "ws1"}); err != nil || !d.Allowed {
		t.Fatalf("AllowRequest: allowed=%v err=%v, want allowed=true err=nil", d.Allowed, err)
	}
	if d, err := l.AllowEnqueue(ctx, EnqueueScope{WorkspaceID: "ws1"}, 100); err != nil || !d.Allowed {
		t.Fatalf("AllowEnqueue: allowed=%v err=%v, want allowed=true err=nil", d.Allowed, err)
	}
	if d, err := l.CheckQueueDepth(ctx, QueueScope{WorkspaceID: "ws1"}, 100000); err != nil || !d.Allowed {
		t.Fatalf("CheckQueueDepth: allowed=%v err=%v, want allowed=true err=nil", d.Allowed, err)
	}
}
