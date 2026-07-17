package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

func TestBuildAudioOverlayFFmpegArgsReplacePadsToVideoDuration(t *testing.T) {
	args, err := buildAudioOverlayFFmpegArgs(audioOverlayFFmpegSpec{
		InputVideoPath:  "/tmp/video.mp4",
		InputAudioPath:  "/tmp/audio.mp3",
		OutputPath:      "/tmp/output.mp4",
		Mode:            "replace",
		Fit:             "trim_to_video",
		VideoDurationMS: 30_000,
		AudioVolume:     80,
		AudioStartMS:    500,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-ss 0.500",
		"[1:a]volume=0.80,apad,atrim=0:30.000[a]",
		"-map 0:v:0",
		"-map [a]",
		"-t 30.000",
		"-movflags +faststart",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %q missing %q", joined, want)
		}
	}
	if strings.Contains(joined, "-shortest") {
		t.Fatalf("replace mode must not use -shortest because short audio would truncate video: %q", joined)
	}
}

func TestBuildAudioOverlayFFmpegArgsMixWithSilentVideoBaseAndLoop(t *testing.T) {
	args, err := buildAudioOverlayFFmpegArgs(audioOverlayFFmpegSpec{
		InputVideoPath:  "/tmp/video.mp4",
		InputAudioPath:  "/tmp/audio.mp3",
		OutputPath:      "/tmp/output.mp4",
		Mode:            "mix",
		Fit:             "loop_to_video",
		VideoDurationMS: 45_000,
		VideoHasAudio:   false,
		VideoVolume:     70,
		AudioVolume:     100,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-stream_loop -1",
		"anullsrc=channel_layout=stereo:sample_rate=44100,atrim=0:45.000[base]",
		"[1:a]volume=1.00,apad,atrim=0:45.000[music]",
		"[base][music]amix=inputs=2:duration=first:normalize=0[a]",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %q missing %q", joined, want)
		}
	}
}

func TestMediaAudioOverlayWorkerProcessesClaimedJob(t *testing.T) {
	queries := newFakeMediaAudioOverlayWorkerQueries()
	store := &fakeMediaAudioOverlayWorkerStorage{}
	processor := &fakeAudioOverlayProcessor{result: audioOverlayProcessResult{
		SizeBytes:  12_345,
		Width:      1080,
		Height:     1920,
		DurationMS: 30_000,
	}}
	worker := NewMediaAudioOverlayWorker(queries, store).WithProcessor(processor)

	worker.runOnce(context.Background())

	if queries.claimCalls != 1 {
		t.Fatalf("claim calls = %d, want 1", queries.claimCalls)
	}
	if queries.promoteCalls != 1 || queries.promoteKind != mediaAudioOverlayKind {
		t.Fatalf("retry promotion calls/kind = %d/%q, want 1/%q", queries.promoteCalls, queries.promoteKind, mediaAudioOverlayKind)
	}
	if queries.claimKind != mediaAudioOverlayKind {
		t.Fatalf("claim kind = %q, want %q", queries.claimKind, mediaAudioOverlayKind)
	}
	if len(store.downloads) != 2 {
		t.Fatalf("downloads = %#v, want video and audio", store.downloads)
	}
	if len(store.uploads) != 1 || !strings.HasSuffix(store.uploads[0].key, ".mp4") {
		t.Fatalf("uploads = %#v, want one mp4 upload", store.uploads)
	}
	if queries.completedSuccessJobID != "mpj_1" || queries.completedSuccessOutputMediaID == "" {
		t.Fatalf("success = %q/%q, want lifecycle-completed job id and output media id", queries.completedSuccessJobID, queries.completedSuccessOutputMediaID)
	}
	if !queries.completedSuccessDeadline.Valid {
		t.Fatal("success cleanup deadline must be plan-aware and non-null")
	}
	wantSuccessDeadline := time.Now().Add(4 * 24 * time.Hour)
	if delta := queries.completedSuccessDeadline.Time.Sub(wantSuccessDeadline); delta < -time.Minute || delta > time.Minute {
		t.Fatalf("basic success cleanup deadline delta = %s, want within one minute of four days", delta)
	}
	if queries.legacySuccessCalls != 0 {
		t.Fatalf("legacy success calls = %d, want 0", queries.legacySuccessCalls)
	}
	if queries.failedJobID != "" {
		t.Fatalf("job should not fail, got failed id %q", queries.failedJobID)
	}
}

func TestMediaAudioOverlayWorkerTerminallyFailsMalformedInputs(t *testing.T) {
	queries := newFakeMediaAudioOverlayWorkerQueries()
	worker := NewMediaAudioOverlayWorker(queries, &fakeMediaAudioOverlayWorkerStorage{}).
		WithProcessor(&fakeAudioOverlayProcessor{})

	err := worker.processJob(context.Background(), db.MediaProcessingJob{
		ID:          "mpj_malformed",
		WorkspaceID: "ws_1",
		Kind:        mediaAudioOverlayKind,
		Status:      "processing",
	})

	if err == nil {
		t.Fatal("process malformed job error = nil, want terminal validation error")
	}
	if queries.failedJobID != "mpj_malformed" {
		t.Fatalf("failed job id = %q, want mpj_malformed", queries.failedJobID)
	}
	if queries.failedErrorCode != "invalid_media_processing_job" {
		t.Fatalf("failed error code = %q, want invalid_media_processing_job", queries.failedErrorCode)
	}
	if queries.failedRetryable {
		t.Fatal("malformed job must not be retryable")
	}
	if queries.completedFailureCalls != 1 || queries.legacyFailureCalls != 0 {
		t.Fatalf("terminal/legacy failure calls = %d/%d, want 1/0", queries.completedFailureCalls, queries.legacyFailureCalls)
	}
	if !queries.completedFailureDeadline.Valid {
		t.Fatal("terminal failure cleanup deadline must be plan-aware and non-null")
	}
}

func TestMediaAudioOverlayWorkerKeepsLifecycleActiveForRetryableFailure(t *testing.T) {
	queries := newFakeMediaAudioOverlayWorkerQueries()
	worker := NewMediaAudioOverlayWorker(queries, &fakeMediaAudioOverlayWorkerStorage{}).
		WithProcessor(&fakeAudioOverlayProcessor{})

	err := worker.processJob(context.Background(), db.MediaProcessingJob{
		ID:                "mpj_retryable",
		WorkspaceID:       "ws_1",
		Kind:              mediaAudioOverlayKind,
		Status:            "processing",
		Attempts:          1,
		InputVideoMediaID: pgtype.Text{String: "missing_video", Valid: true},
		InputAudioMediaID: pgtype.Text{String: "med_audio", Valid: true},
	})

	if err == nil {
		t.Fatal("retryable input failure error = nil")
	}
	if queries.requeueCalls != 1 || queries.requeuedJobID != "mpj_retryable" {
		t.Fatalf("requeue calls/job = %d/%q, want 1/mpj_retryable", queries.requeueCalls, queries.requeuedJobID)
	}
	if queries.completedFailureCalls != 0 || queries.legacyFailureCalls != 0 {
		t.Fatalf("terminal/legacy failure calls = %d/%d, want 0/0 so input usages stay active", queries.completedFailureCalls, queries.legacyFailureCalls)
	}
}

func TestMediaAudioOverlayWorkerTerminallyFailsAfterRetryAttemptsExhausted(t *testing.T) {
	queries := newFakeMediaAudioOverlayWorkerQueries()
	worker := NewMediaAudioOverlayWorker(queries, &fakeMediaAudioOverlayWorkerStorage{}).
		WithProcessor(&fakeAudioOverlayProcessor{})

	err := worker.processJob(context.Background(), db.MediaProcessingJob{
		ID:                "mpj_exhausted",
		WorkspaceID:       "ws_1",
		Kind:              mediaAudioOverlayKind,
		Status:            "processing",
		Attempts:          3,
		InputVideoMediaID: pgtype.Text{String: "missing_video", Valid: true},
		InputAudioMediaID: pgtype.Text{String: "med_audio", Valid: true},
	})

	if err == nil {
		t.Fatal("exhausted retry failure error = nil")
	}
	if queries.requeueCalls != 0 || queries.completedFailureCalls != 1 {
		t.Fatalf("requeue/terminal calls = %d/%d, want 0/1", queries.requeueCalls, queries.completedFailureCalls)
	}
	if queries.failedErrorCode != "media_processing_attempts_exhausted" {
		t.Fatalf("terminal code = %q, want media_processing_attempts_exhausted", queries.failedErrorCode)
	}
}

type fakeMediaAudioOverlayWorkerQueries struct {
	claimCalls                    int
	claimKind                     string
	legacySuccessCalls            int
	completedSuccessJobID         string
	completedSuccessOutputMediaID string
	completedSuccessDeadline      pgtype.Timestamptz
	failedJobID                   string
	failedErrorCode               string
	failedRetryable               bool
	legacyFailureCalls            int
	completedFailureCalls         int
	completedFailureDeadline      pgtype.Timestamptz
	requeueCalls                  int
	requeuedJobID                 string
	promoteCalls                  int
	promoteKind                   string
}

func newFakeMediaAudioOverlayWorkerQueries() *fakeMediaAudioOverlayWorkerQueries {
	return &fakeMediaAudioOverlayWorkerQueries{}
}

func (f *fakeMediaAudioOverlayWorkerQueries) ClaimMediaProcessingJobsByKind(_ context.Context, arg db.ClaimMediaProcessingJobsByKindParams) ([]db.MediaProcessingJob, error) {
	f.claimCalls++
	f.claimKind = arg.JobKind
	now := pgtype.Timestamptz{Time: time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), Valid: true}
	return []db.MediaProcessingJob{{
		ID:                "mpj_1",
		WorkspaceID:       "ws_1",
		Kind:              "audio_overlay",
		Status:            "processing",
		InputVideoMediaID: pgtype.Text{String: "med_video", Valid: true},
		InputAudioMediaID: pgtype.Text{String: "med_audio", Valid: true},
		Mode:              "mix",
		Fit:               "trim_to_video",
		VideoVolume:       70,
		AudioVolume:       100,
		AudioStartMs:      0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) PromoteDueMediaProcessingRetriesByKind(_ context.Context, kind string) (int64, error) {
	f.promoteCalls++
	f.promoteKind = kind
	return 0, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) GetMediaByIDAndWorkspace(_ context.Context, arg db.GetMediaByIDAndWorkspaceParams) (db.Media, error) {
	switch arg.ID {
	case "med_video":
		return db.Media{
			ID:          "med_video",
			WorkspaceID: arg.WorkspaceID,
			StorageKey:  "media/video.mp4",
			ContentType: "video/mp4",
			Status:      "uploaded",
			SizeBytes:   40_000_000,
			DurationMs:  pgtype.Int4{Int32: 30_000, Valid: true},
		}, nil
	case "med_audio":
		return db.Media{
			ID:          "med_audio",
			WorkspaceID: arg.WorkspaceID,
			StorageKey:  "media/audio.mp3",
			ContentType: "audio/mpeg",
			Status:      "uploaded",
			SizeBytes:   5_000_000,
		}, nil
	default:
		return db.Media{}, storage.ErrNotConfigured
	}
}

func (f *fakeMediaAudioOverlayWorkerQueries) CreateMedia(_ context.Context, arg db.CreateMediaParams) (db.Media, error) {
	return db.Media{
		ID:          "med_output",
		WorkspaceID: arg.WorkspaceID,
		StorageKey:  arg.StorageKey,
		ContentType: arg.ContentType,
		SizeBytes:   arg.SizeBytes,
		Status:      arg.Status,
	}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) UpdateMediaStorageKey(_ context.Context, arg db.UpdateMediaStorageKeyParams) (db.Media, error) {
	return db.Media{
		ID:         arg.ID,
		StorageKey: arg.StorageKey,
	}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) MarkMediaUploaded(_ context.Context, arg db.MarkMediaUploadedParams) (db.Media, error) {
	return db.Media{
		ID:          arg.ID,
		StorageKey:  "media/med_output.mp4",
		ContentType: arg.ContentType,
		SizeBytes:   arg.SizeBytes,
		Status:      "uploaded",
		Width:       arg.Width,
		Height:      arg.Height,
		DurationMs:  arg.DurationMs,
	}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) HardDeleteMedia(context.Context, string) error {
	return nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) MarkMediaProcessingJobSucceeded(_ context.Context, arg db.MarkMediaProcessingJobSucceededParams) (db.MediaProcessingJob, error) {
	f.legacySuccessCalls++
	return db.MediaProcessingJob{ID: arg.ID, OutputMediaID: arg.OutputMediaID, Status: "succeeded"}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) MarkMediaProcessingJobFailed(_ context.Context, arg db.MarkMediaProcessingJobFailedParams) (db.MediaProcessingJob, error) {
	f.legacyFailureCalls++
	f.failedJobID = arg.ID
	f.failedErrorCode = arg.ErrorCode.String
	f.failedRetryable = arg.Retryable
	return db.MediaProcessingJob{ID: arg.ID, Status: "failed"}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) CompleteMediaProcessingJobSucceeded(_ context.Context, arg db.CompleteMediaProcessingJobSucceededParams) (db.MediaProcessingJob, error) {
	f.completedSuccessJobID = arg.JobID
	f.completedSuccessOutputMediaID = arg.OutputMediaID.String
	f.completedSuccessDeadline = arg.CleanupAfterAt
	return db.MediaProcessingJob{ID: arg.JobID, OutputMediaID: arg.OutputMediaID, Status: "succeeded"}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) CompleteMediaProcessingJobFailed(_ context.Context, arg db.CompleteMediaProcessingJobFailedParams) (db.MediaProcessingJob, error) {
	f.completedFailureCalls++
	f.failedJobID = arg.JobID
	f.failedErrorCode = arg.ErrorCode.String
	f.failedRetryable = false
	f.completedFailureDeadline = arg.CleanupAfterAt
	return db.MediaProcessingJob{ID: arg.JobID, Status: "failed"}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) GetSubscriptionByWorkspace(context.Context, string) (db.Subscription, error) {
	return db.Subscription{WorkspaceID: "ws_1", PlanID: "basic"}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) RequeueMediaProcessingJob(_ context.Context, arg db.RequeueMediaProcessingJobParams) (db.MediaProcessingJob, error) {
	f.requeueCalls++
	f.requeuedJobID = arg.JobID
	return db.MediaProcessingJob{ID: arg.JobID, Status: "retry_wait", Retryable: true}, nil
}

type fakeMediaAudioOverlayWorkerStorage struct {
	downloads []string
	uploads   []struct {
		key string
	}
}

func (f *fakeMediaAudioOverlayWorkerStorage) DownloadObject(_ context.Context, key, _ string) error {
	f.downloads = append(f.downloads, key)
	return nil
}

func (f *fakeMediaAudioOverlayWorkerStorage) PutFile(_ context.Context, key, _, _, _ string) error {
	f.uploads = append(f.uploads, struct{ key string }{key: key})
	return nil
}

func (f *fakeMediaAudioOverlayWorkerStorage) Head(context.Context, string) (storage.HeadResult, error) {
	return storage.HeadResult{Exists: true, ContentType: "video/mp4", SizeBytes: 12_345}, nil
}

func (f *fakeMediaAudioOverlayWorkerStorage) ProbeVideo(context.Context, string) (storage.VideoMetadata, error) {
	return storage.VideoMetadata{Width: 1080, Height: 1920, DurationMS: 30_000}, nil
}

type fakeAudioOverlayProcessor struct {
	result audioOverlayProcessResult
	err    error
}

func (f *fakeAudioOverlayProcessor) Process(context.Context, audioOverlayProcessRequest) (audioOverlayProcessResult, error) {
	return f.result, f.err
}
