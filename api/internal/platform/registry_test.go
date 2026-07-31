package platform

import (
	"context"
	"testing"
	"time"
)

type accountStateInvalidatorAdapter struct {
	platformName string
	accountID    string
}

func (a *accountStateInvalidatorAdapter) Platform() string { return a.platformName }
func (a *accountStateInvalidatorAdapter) Connect(context.Context, map[string]string) (*ConnectResult, error) {
	return nil, nil
}
func (a *accountStateInvalidatorAdapter) Post(context.Context, string, string, []MediaItem, map[string]any) (*PostResult, error) {
	return nil, nil
}
func (a *accountStateInvalidatorAdapter) DeletePost(context.Context, string, string) error {
	return nil
}
func (a *accountStateInvalidatorAdapter) RefreshToken(context.Context, string) (string, string, time.Time, error) {
	return "", "", time.Time{}, nil
}
func (a *accountStateInvalidatorAdapter) InvalidateAccountState(accountID string) {
	a.accountID = accountID
}

func TestInvalidateAccountStateDelegatesToRegisteredAdapter(t *testing.T) {
	adapter := &accountStateInvalidatorAdapter{platformName: "account_state_test"}
	Register(adapter)
	InvalidateAccountState("account_state_test", "sa_123")
	if adapter.accountID != "sa_123" {
		t.Fatalf("invalidated account = %q, want sa_123", adapter.accountID)
	}
}

func TestPinterestInvalidateAccountStateClearsEveryEnvironment(t *testing.T) {
	adapter := &PinterestAdapter{}
	expires := time.Now().Add(time.Minute)
	adapter.storePinterestBoardCache(pinterestBoardCacheKey{AccountID: "sa_1", Environment: "production"}, pinterestBoardCacheEntry{TokenFingerprint: "token", ExpiresAt: expires})
	adapter.storePinterestBoardCache(pinterestBoardCacheKey{AccountID: "sa_1", Environment: "sandbox"}, pinterestBoardCacheEntry{TokenFingerprint: "token", ExpiresAt: expires})
	adapter.storePinterestBoardCache(pinterestBoardCacheKey{AccountID: "sa_2", Environment: "production"}, pinterestBoardCacheEntry{TokenFingerprint: "token", ExpiresAt: expires})

	adapter.InvalidateAccountState("sa_1")
	if _, ok := adapter.loadPinterestBoardCache(pinterestBoardCacheKey{AccountID: "sa_1", Environment: "production"}, "token"); ok {
		t.Fatal("production cache was not invalidated")
	}
	if _, ok := adapter.loadPinterestBoardCache(pinterestBoardCacheKey{AccountID: "sa_1", Environment: "sandbox"}, "token"); ok {
		t.Fatal("sandbox cache was not invalidated")
	}
	if _, ok := adapter.loadPinterestBoardCache(pinterestBoardCacheKey{AccountID: "sa_2", Environment: "production"}, "token"); !ok {
		t.Fatal("another account cache was invalidated")
	}
}
