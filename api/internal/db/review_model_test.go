package db

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestReviewSQLCModelsExposeRequiredFields(t *testing.T) {
	domainParams := CreateReviewDomainParams{
		WorkspaceID:       "ws_1",
		Domain:            "review.example.com",
		Provider:          pgtype.Text{String: "manual", Valid: true},
		Status:            "pending",
		VerificationToken: "unipost-review=rv_test",
		CnameTarget:       "review.unipost.dev",
		TlsStatus:         "pending",
	}
	kitParams := CreateReviewKitParams{
		WorkspaceID:    "ws_1",
		Platform:       "tiktok",
		UseCase:        "content_posting",
		ReviewDomainID: "rvdom_1",
		BrandSnapshot:  []byte(`{"display_name":"Acme"}`),
		RequiredScopes: []string{"user.info.basic", "video.publish", "video.upload"},
		Status:         "draft",
	}
	jobParams := CreateReviewJobParams{
		ReviewKitID:          "rvkit_1",
		WorkspaceID:          "ws_1",
		Platform:             "tiktok",
		Status:               "queued",
		AgentVersion:         pgtype.Text{String: "0.1.0", Valid: true},
		ReviewSessionTokenID: pgtype.Text{String: "rvsess_1", Valid: true},
	}
	sessionParams := CreateReviewSessionParams{
		ReviewJobID:  "rvjob_1",
		ReviewKitID:  "rvkit_1",
		WorkspaceID:  "ws_1",
		Platform:     "tiktok",
		ReviewDomain: "review.example.com",
		TokenHash:    "hash",
		ExpiresAt:    pgtype.Timestamptz{Valid: true},
	}
	agentTokenParams := CreateReviewAgentTokenParams{
		ReviewJobID: "rvjob_1",
		WorkspaceID: "ws_1",
		Platform:    "tiktok",
		TokenHash:   "agent_hash",
		ExpiresAt:   pgtype.Timestamptz{Valid: true},
	}

	if domainParams.WorkspaceID != "ws_1" || kitParams.Platform != "tiktok" || jobParams.Platform != "tiktok" || sessionParams.Platform != "tiktok" || agentTokenParams.Platform != "tiktok" {
		t.Fatal("review sqlc params should preserve TikTok platform fields")
	}
}
