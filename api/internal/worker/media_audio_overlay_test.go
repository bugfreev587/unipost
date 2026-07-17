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
	if queries.claimKind != mediaAudioOverlayKind {
		t.Fatalf("claim kind = %q, want %q", queries.claimKind, mediaAudioOverlayKind)
	}
	if len(store.downloads) != 2 {
		t.Fatalf("downloads = %#v, want video and audio", store.downloads)
	}
	if len(store.uploads) != 1 || !strings.HasSuffix(store.uploads[0].key, ".mp4") {
		t.Fatalf("uploads = %#v, want one mp4 upload", store.uploads)
	}
	if queries.succeededJobID != "mpj_1" || queries.succeededOutputMediaID == "" {
		t.Fatalf("success = %q/%q, want job id and output media id", queries.succeededJobID, queries.succeededOutputMediaID)
	}
	if queries.failedJobID != "" {
		t.Fatalf("job should not fail, got failed id %q", queries.failedJobID)
	}
}

type fakeMediaAudioOverlayWorkerQueries struct {
	claimCalls             int
	claimKind              string
	succeededJobID         string
	succeededOutputMediaID string
	failedJobID            string
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
	f.succeededJobID = arg.ID
	f.succeededOutputMediaID = arg.OutputMediaID.String
	return db.MediaProcessingJob{ID: arg.ID, OutputMediaID: arg.OutputMediaID, Status: "succeeded"}, nil
}

func (f *fakeMediaAudioOverlayWorkerQueries) MarkMediaProcessingJobFailed(_ context.Context, arg db.MarkMediaProcessingJobFailedParams) (db.MediaProcessingJob, error) {
	f.failedJobID = arg.ID
	return db.MediaProcessingJob{ID: arg.ID, Status: "failed"}, nil
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
