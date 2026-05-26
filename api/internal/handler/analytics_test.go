package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestCachedPostAnalyticsFreshRejectsCacheFetchedBeforeAccountRefresh(t *testing.T) {
	now := time.Date(2026, 5, 26, 5, 30, 0, 0, time.UTC)
	cached := db.PostAnalytic{
		FetchedAt: pgtype.Timestamptz{Time: now.Add(-20 * time.Minute), Valid: true},
	}
	account := db.SocialAccount{
		LastRefreshedAt: pgtype.Timestamptz{Time: now.Add(-10 * time.Minute), Valid: true},
	}

	if cachedPostAnalyticsFresh(cached, account, false, now) {
		t.Fatal("cache fetched before account refresh should be stale")
	}
}

func TestCachedPostAnalyticsFreshAcceptsRecentCacheAfterAccountRefresh(t *testing.T) {
	now := time.Date(2026, 5, 26, 5, 30, 0, 0, time.UTC)
	cached := db.PostAnalytic{
		FetchedAt: pgtype.Timestamptz{Time: now.Add(-5 * time.Minute), Valid: true},
	}
	account := db.SocialAccount{
		LastRefreshedAt: pgtype.Timestamptz{Time: now.Add(-10 * time.Minute), Valid: true},
	}

	if !cachedPostAnalyticsFresh(cached, account, false, now) {
		t.Fatal("recent cache fetched after account refresh should be fresh")
	}
}

func TestCachedPostAnalyticsFreshRejectsForceRefresh(t *testing.T) {
	now := time.Date(2026, 5, 26, 5, 30, 0, 0, time.UTC)
	cached := db.PostAnalytic{
		FetchedAt: pgtype.Timestamptz{Time: now.Add(-5 * time.Minute), Valid: true},
	}

	if cachedPostAnalyticsFresh(cached, db.SocialAccount{}, true, now) {
		t.Fatal("force refresh should bypass cache")
	}
}
