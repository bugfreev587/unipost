package db

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestMediaProcessingJobSQLCModelsExposeRequiredFields(t *testing.T) {
	createParams := CreateMediaProcessingJobParams{
		WorkspaceID:       "ws_1",
		Kind:              "audio_overlay",
		Status:            "queued",
		InputVideoMediaID: "med_video",
		InputAudioMediaID: "med_audio",
		OutputMediaID:     pgtype.Text{},
		IdempotencyKey:    pgtype.Text{String: "overlay-1", Valid: true},
		RequestHash:       pgtype.Text{String: "hash", Valid: true},
		Mode:              "mix",
		Fit:               "trim_to_video",
		VideoVolume:       70,
		AudioVolume:       100,
		AudioStartMs:      0,
		RequestJson:       []byte(`{"kind":"audio_overlay"}`),
	}
	job := MediaProcessingJob{
		ID:                "mpj_1",
		WorkspaceID:       createParams.WorkspaceID,
		Kind:              createParams.Kind,
		Status:            createParams.Status,
		InputVideoMediaID: createParams.InputVideoMediaID,
		InputAudioMediaID: createParams.InputAudioMediaID,
		Mode:              createParams.Mode,
		Fit:               createParams.Fit,
		VideoVolume:       createParams.VideoVolume,
		AudioVolume:       createParams.AudioVolume,
		AudioStartMs:      createParams.AudioStartMs,
		Request:           createParams.RequestJson,
	}

	if job.Kind != "audio_overlay" || job.Status != "queued" || job.InputVideoMediaID != "med_video" || job.InputAudioMediaID != "med_audio" {
		t.Fatalf("media processing job model lost required media references: %#v", job)
	}
}
