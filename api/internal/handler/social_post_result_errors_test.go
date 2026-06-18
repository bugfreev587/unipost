package handler

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestPostResultResponseIncludesStructuredFailureFields(t *testing.T) {
	result := db.SocialPostResult{
		ID:                "spr_1",
		SocialAccountID:   "acc_tiktok",
		Caption:           "launch photo",
		Status:            "failed",
		ErrorMessage:      pgtype.Text{String: "TikTok rejected the photo metadata.", Valid: true},
		ErrorCode:         pgtype.Text{String: "platform_request_invalid", Valid: true},
		FailureStage:      pgtype.Text{String: "platform_publish_init", Valid: true},
		PlatformErrorCode: pgtype.Text{String: "invalid_params", Valid: true},
		IsRetriable:       pgtype.Bool{Bool: false, Valid: true},
		NextAction:        pgtype.Text{String: "review_platform_options", Valid: true},
	}

	response := postResultResponseFromDBResult(result, accountSummary{
		Platform: "tiktok",
		Name:     "TailTales",
	})

	body, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got["error_message"] != "TikTok rejected the photo metadata." {
		t.Fatalf("error_message = %#v", got["error_message"])
	}
	if got["error_code"] != "platform_request_invalid" {
		t.Fatalf("error_code = %#v", got["error_code"])
	}
	if got["failure_stage"] != "platform_publish_init" {
		t.Fatalf("failure_stage = %#v", got["failure_stage"])
	}
	if got["platform_error_code"] != "invalid_params" {
		t.Fatalf("platform_error_code = %#v", got["platform_error_code"])
	}
	if got["is_retriable"] != false {
		t.Fatalf("is_retriable = %#v", got["is_retriable"])
	}
	if got["next_action"] != "review_platform_options" {
		t.Fatalf("next_action = %#v", got["next_action"])
	}
}
