package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/mediaretention"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

const (
	mediaAudioOverlayWorkerInterval = 5 * time.Second
	mediaAudioOverlayClaimBatch     = 3
	mediaAudioOverlayKind           = "audio_overlay"
	mediaAudioOverlayOutputType     = "video/mp4"
)

type mediaAudioOverlayWorkerQueries interface {
	PromoteDueMediaProcessingRetriesByKind(context.Context, string) (int64, error)
	ClaimMediaProcessingJobsByKind(context.Context, db.ClaimMediaProcessingJobsByKindParams) ([]db.MediaProcessingJob, error)
	GetMediaByIDAndWorkspace(context.Context, db.GetMediaByIDAndWorkspaceParams) (db.Media, error)
	CreateMedia(context.Context, db.CreateMediaParams) (db.Media, error)
	UpdateMediaStorageKey(context.Context, db.UpdateMediaStorageKeyParams) (db.Media, error)
	MarkMediaUploaded(context.Context, db.MarkMediaUploadedParams) (db.Media, error)
	HardDeleteMedia(context.Context, string) error
	RequeueMediaProcessingJob(context.Context, db.RequeueMediaProcessingJobParams) (db.MediaProcessingJob, error)
	CompleteMediaProcessingJobSucceeded(context.Context, db.CompleteMediaProcessingJobSucceededParams) (db.MediaProcessingJob, error)
	CompleteMediaProcessingJobFailed(context.Context, db.CompleteMediaProcessingJobFailedParams) (db.MediaProcessingJob, error)
	GetSubscriptionByWorkspace(context.Context, string) (db.Subscription, error)
}

type mediaAudioOverlayWorkerStorage interface {
	DownloadObject(context.Context, string, string) error
	PutFile(context.Context, string, string, string, string) error
	Head(context.Context, string) (storage.HeadResult, error)
	ProbeVideo(context.Context, string) (storage.VideoMetadata, error)
}

type audioOverlayProcessor interface {
	Process(context.Context, audioOverlayProcessRequest) (audioOverlayProcessResult, error)
}

type MediaAudioOverlayWorker struct {
	queries   mediaAudioOverlayWorkerQueries
	storage   mediaAudioOverlayWorkerStorage
	processor audioOverlayProcessor
}

func NewMediaAudioOverlayWorker(queries mediaAudioOverlayWorkerQueries, store mediaAudioOverlayWorkerStorage) *MediaAudioOverlayWorker {
	return &MediaAudioOverlayWorker{
		queries:   queries,
		storage:   store,
		processor: newFFmpegAudioOverlayProcessor(),
	}
}

func (w *MediaAudioOverlayWorker) WithProcessor(processor audioOverlayProcessor) *MediaAudioOverlayWorker {
	w.processor = processor
	return w
}

func (w *MediaAudioOverlayWorker) Start(ctx context.Context) {
	if w.storage == nil {
		slog.Info("media audio overlay worker: storage not configured, worker disabled")
		return
	}
	if w.processor == nil {
		slog.Info("media audio overlay worker: processor not configured, worker disabled")
		return
	}

	ticker := time.NewTicker(mediaAudioOverlayWorkerInterval)
	defer ticker.Stop()

	slog.Info("media audio overlay worker started", "interval", mediaAudioOverlayWorkerInterval)
	for {
		select {
		case <-ctx.Done():
			slog.Info("media audio overlay worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *MediaAudioOverlayWorker) runOnce(ctx context.Context) {
	if w.queries == nil || w.storage == nil || w.processor == nil {
		return
	}
	if _, err := w.queries.PromoteDueMediaProcessingRetriesByKind(ctx, mediaAudioOverlayKind); err != nil {
		slog.Error("media audio overlay worker: retry promotion failed", "error", err)
		return
	}
	jobs, err := w.queries.ClaimMediaProcessingJobsByKind(ctx, db.ClaimMediaProcessingJobsByKindParams{
		JobKind:    mediaAudioOverlayKind,
		BatchLimit: mediaAudioOverlayClaimBatch,
	})
	if err != nil {
		slog.Error("media audio overlay worker: claim failed", "error", err)
		return
	}
	for _, job := range jobs {
		if err := w.processJob(ctx, job); err != nil {
			slog.Error("media audio overlay worker: process failed", "job_id", job.ID, "error", err)
		}
	}
}

func (w *MediaAudioOverlayWorker) processJob(ctx context.Context, job db.MediaProcessingJob) error {
	if job.Kind != mediaAudioOverlayKind {
		return fmt.Errorf("unsupported media processing job kind %q", job.Kind)
	}
	if !job.InputVideoMediaID.Valid || strings.TrimSpace(job.InputVideoMediaID.String) == "" {
		return w.failJob(ctx, job, "invalid_media_processing_job", "input video media id is missing", false)
	}
	if !job.InputAudioMediaID.Valid || strings.TrimSpace(job.InputAudioMediaID.String) == "" {
		return w.failJob(ctx, job, "invalid_media_processing_job", "input audio media id is missing", false)
	}

	video, err := w.queries.GetMediaByIDAndWorkspace(ctx, db.GetMediaByIDAndWorkspaceParams{
		ID:          job.InputVideoMediaID.String,
		WorkspaceID: job.WorkspaceID,
	})
	if err != nil {
		return w.failJob(ctx, job, "input_media_unavailable", "input video media is unavailable", true)
	}
	audio, err := w.queries.GetMediaByIDAndWorkspace(ctx, db.GetMediaByIDAndWorkspaceParams{
		ID:          job.InputAudioMediaID.String,
		WorkspaceID: job.WorkspaceID,
	})
	if err != nil {
		return w.failJob(ctx, job, "input_media_unavailable", "input audio media is unavailable", true)
	}

	tmpDir, err := os.MkdirTemp("", "unipost-audio-overlay-*")
	if err != nil {
		return w.failJob(ctx, job, "audio_overlay_processing_failed", "failed to create processing workspace", true)
	}
	defer os.RemoveAll(tmpDir)

	videoPath := filepath.Join(tmpDir, "input_video"+mediaExt(video.StorageKey, ".mp4"))
	audioPath := filepath.Join(tmpDir, "input_audio"+mediaExt(audio.StorageKey, ".bin"))
	outputPath := filepath.Join(tmpDir, "output.mp4")

	if err := w.storage.DownloadObject(ctx, video.StorageKey, videoPath); err != nil {
		return w.failJob(ctx, job, "input_media_unavailable", "input video object is unavailable", true)
	}
	if err := w.storage.DownloadObject(ctx, audio.StorageKey, audioPath); err != nil {
		return w.failJob(ctx, job, "input_media_unavailable", "input audio object is unavailable", true)
	}

	result, err := w.processor.Process(ctx, audioOverlayProcessRequest{
		Job:             job,
		Video:           video,
		Audio:           audio,
		InputVideoPath:  videoPath,
		InputAudioPath:  audioPath,
		OutputVideoPath: outputPath,
	})
	if err != nil {
		return w.failJob(ctx, job, "audio_overlay_processing_failed", err.Error(), true)
	}

	outputMedia, err := w.createOutputMedia(ctx, job, outputPath, result)
	if err != nil {
		return w.failJob(ctx, job, "audio_overlay_output_upload_failed", err.Error(), true)
	}

	cleanupAfter, err := w.processingCleanupDeadline(ctx, job.WorkspaceID, "published")
	if err != nil {
		return fmt.Errorf("calculate media processing success retention: %w", err)
	}
	if _, err := w.queries.CompleteMediaProcessingJobSucceeded(ctx, db.CompleteMediaProcessingJobSucceededParams{
		JobID:          job.ID,
		OutputMediaID:  pgtype.Text{String: outputMedia.ID, Valid: true},
		CleanupAfterAt: cleanupAfter,
	}); err != nil {
		return fmt.Errorf("complete media processing job succeeded: %w", err)
	}
	return nil
}

func (w *MediaAudioOverlayWorker) createOutputMedia(ctx context.Context, job db.MediaProcessingJob, outputPath string, result audioOverlayProcessResult) (db.Media, error) {
	row, err := w.queries.CreateMedia(ctx, db.CreateMediaParams{
		WorkspaceID: job.WorkspaceID,
		StorageKey:  "placeholder/" + uuid.NewString(),
		ContentType: mediaAudioOverlayOutputType,
		SizeBytes:   result.SizeBytes,
		Status:      "pending",
		ContentHash: pgtype.Text{},
	})
	if err != nil {
		return db.Media{}, fmt.Errorf("create output media row: %w", err)
	}

	finalKey := storage.MediaKey(row.ID, ".mp4")
	updated, err := w.queries.UpdateMediaStorageKey(ctx, db.UpdateMediaStorageKeyParams{
		ID:         row.ID,
		StorageKey: finalKey,
	})
	if err != nil {
		_ = w.queries.HardDeleteMedia(ctx, row.ID)
		return db.Media{}, fmt.Errorf("update output media key: %w", err)
	}
	if err := w.storage.PutFile(ctx, finalKey, outputPath, mediaAudioOverlayOutputType, "public, max-age=31536000, immutable"); err != nil {
		_ = w.queries.HardDeleteMedia(ctx, row.ID)
		return db.Media{}, fmt.Errorf("upload output media: %w", err)
	}

	head, err := w.storage.Head(ctx, finalKey)
	if err != nil || !head.Exists {
		_ = w.queries.HardDeleteMedia(ctx, row.ID)
		return db.Media{}, fmt.Errorf("head output media: %w", err)
	}
	meta, probeErr := w.storage.ProbeVideo(ctx, finalKey)
	if probeErr != nil {
		slog.Warn("media audio overlay worker: output probe failed", "job_id", job.ID, "media_id", row.ID, "error", probeErr)
	}
	if meta.Width == 0 {
		meta.Width = result.Width
	}
	if meta.Height == 0 {
		meta.Height = result.Height
	}
	if meta.DurationMS == 0 {
		meta.DurationMS = result.DurationMS
	}

	marked, err := w.queries.MarkMediaUploaded(ctx, db.MarkMediaUploadedParams{
		ID:          updated.ID,
		SizeBytes:   head.SizeBytes,
		ContentType: pickOutputContentType(head.ContentType),
		Width:       int4FromPositive(meta.Width),
		Height:      int4FromPositive(meta.Height),
		DurationMs:  int4FromPositive(meta.DurationMS),
	})
	if err != nil {
		_ = w.queries.HardDeleteMedia(ctx, row.ID)
		return db.Media{}, fmt.Errorf("mark output media uploaded: %w", err)
	}
	return marked, nil
}

func (w *MediaAudioOverlayWorker) failJob(ctx context.Context, job db.MediaProcessingJob, code, message string, retryable bool) error {
	if retryable && job.Attempts < 3 {
		_, err := w.queries.RequeueMediaProcessingJob(ctx, db.RequeueMediaProcessingJobParams{
			ErrorCode:    pgtype.Text{String: code, Valid: true},
			ErrorMessage: pgtype.Text{String: message, Valid: true},
			JobID:        job.ID,
		})
		if err != nil {
			return fmt.Errorf("%s: requeue retryable failure: %w", message, err)
		}
		return fmt.Errorf("%s", message)
	}

	if retryable {
		code = "media_processing_attempts_exhausted"
		message = fmt.Sprintf("processing attempts exhausted: %s", message)
	}

	cleanupAfter, deadlineErr := w.processingCleanupDeadline(ctx, job.WorkspaceID, "failed")
	if deadlineErr != nil {
		return fmt.Errorf("%s: calculate media processing failure retention: %w", message, deadlineErr)
	}
	_, err := w.queries.CompleteMediaProcessingJobFailed(ctx, db.CompleteMediaProcessingJobFailedParams{
		JobID:          job.ID,
		ErrorCode:      pgtype.Text{String: code, Valid: true},
		ErrorMessage:   pgtype.Text{String: message, Valid: true},
		CleanupAfterAt: cleanupAfter,
	})
	if err != nil {
		return fmt.Errorf("%s: complete terminal failure: %w", message, err)
	}
	return fmt.Errorf("%s", message)
}

func (w *MediaAudioOverlayWorker) processingCleanupDeadline(ctx context.Context, workspaceID, terminalStatus string) (pgtype.Timestamptz, error) {
	planID := "free"
	subscription, err := w.queries.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err == nil {
		planID = subscription.PlanID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.Timestamptz{}, err
	}

	retention, ok := mediaretention.RetentionForPlanStatus(planID, terminalStatus)
	if !ok {
		return pgtype.Timestamptz{}, fmt.Errorf("unsupported terminal status %q", terminalStatus)
	}
	return pgtype.Timestamptz{Time: time.Now().Add(retention), Valid: true}, nil
}

type audioOverlayProcessRequest struct {
	Job             db.MediaProcessingJob
	Video           db.Media
	Audio           db.Media
	InputVideoPath  string
	InputAudioPath  string
	OutputVideoPath string
}

type audioOverlayProcessResult struct {
	SizeBytes  int64
	Width      int
	Height     int
	DurationMS int
}

type audioOverlayFFmpegSpec struct {
	InputVideoPath  string
	InputAudioPath  string
	OutputPath      string
	Mode            string
	Fit             string
	VideoDurationMS int
	VideoHasAudio   bool
	VideoVolume     int32
	AudioVolume     int32
	AudioStartMS    int32
}

type ffmpegAudioOverlayProcessor struct {
	ffmpegPath  string
	ffprobePath string
}

func newFFmpegAudioOverlayProcessor() *ffmpegAudioOverlayProcessor {
	ffmpegPath := strings.TrimSpace(os.Getenv("FFMPEG_PATH"))
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	ffprobePath := strings.TrimSpace(os.Getenv("FFPROBE_PATH"))
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	return &ffmpegAudioOverlayProcessor{ffmpegPath: ffmpegPath, ffprobePath: ffprobePath}
}

func (p *ffmpegAudioOverlayProcessor) Process(ctx context.Context, req audioOverlayProcessRequest) (audioOverlayProcessResult, error) {
	probe, err := p.probeInput(ctx, req.InputVideoPath)
	if err != nil {
		return audioOverlayProcessResult{}, err
	}
	durationMS := probe.DurationMS
	if durationMS <= 0 && req.Video.DurationMs.Valid {
		durationMS = int(req.Video.DurationMs.Int32)
	}
	if durationMS <= 0 {
		return audioOverlayProcessResult{}, fmt.Errorf("ffprobe_failed: input video duration is unknown")
	}

	args, err := buildAudioOverlayFFmpegArgs(audioOverlayFFmpegSpec{
		InputVideoPath:  req.InputVideoPath,
		InputAudioPath:  req.InputAudioPath,
		OutputPath:      req.OutputVideoPath,
		Mode:            req.Job.Mode,
		Fit:             req.Job.Fit,
		VideoDurationMS: durationMS,
		VideoHasAudio:   probe.HasAudio,
		VideoVolume:     req.Job.VideoVolume,
		AudioVolume:     req.Job.AudioVolume,
		AudioStartMS:    req.Job.AudioStartMs,
	})
	if err != nil {
		return audioOverlayProcessResult{}, err
	}
	cmd := exec.CommandContext(ctx, p.ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return audioOverlayProcessResult{}, fmt.Errorf("ffmpeg failed: %w: %s", err, string(out))
	}
	stat, err := os.Stat(req.OutputVideoPath)
	if err != nil {
		return audioOverlayProcessResult{}, fmt.Errorf("stat output video: %w", err)
	}
	return audioOverlayProcessResult{SizeBytes: stat.Size(), DurationMS: durationMS}, nil
}

type ffprobeInputResult struct {
	DurationMS int
	HasAudio   bool
}

func (p *ffmpegAudioOverlayProcessor) probeInput(ctx context.Context, path string) (ffprobeInputResult, error) {
	cmd := exec.CommandContext(ctx, p.ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration:stream=codec_type",
		"-of", "json",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return ffprobeInputResult{}, fmt.Errorf("ffprobe_failed: %w", err)
	}
	var payload struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return ffprobeInputResult{}, fmt.Errorf("ffprobe_failed: parse output: %w", err)
	}
	var res ffprobeInputResult
	for _, stream := range payload.Streams {
		if stream.CodecType == "audio" {
			res.HasAudio = true
			break
		}
	}
	if payload.Format.Duration != "" {
		var seconds float64
		if _, err := fmt.Sscanf(payload.Format.Duration, "%f", &seconds); err == nil && seconds > 0 {
			res.DurationMS = int(seconds * 1000)
		}
	}
	return res, nil
}

func buildAudioOverlayFFmpegArgs(spec audioOverlayFFmpegSpec) ([]string, error) {
	if spec.InputVideoPath == "" || spec.InputAudioPath == "" || spec.OutputPath == "" {
		return nil, fmt.Errorf("audio overlay ffmpeg paths are required")
	}
	if spec.VideoDurationMS <= 0 {
		return nil, fmt.Errorf("video duration is required")
	}
	duration := secondsString(spec.VideoDurationMS)
	audioOffset := secondsString(int(spec.AudioStartMS))
	videoGain := volumeGain(spec.VideoVolume)
	audioGain := volumeGain(spec.AudioVolume)

	args := []string{"-y", "-i", spec.InputVideoPath}
	if spec.Fit == "loop_to_video" {
		args = append(args, "-stream_loop", "-1")
	}
	if spec.AudioStartMS > 0 {
		args = append(args, "-ss", audioOffset)
	}
	args = append(args, "-i", spec.InputAudioPath)

	var filter string
	switch spec.Mode {
	case "replace":
		filter = fmt.Sprintf("[1:a]volume=%s,apad,atrim=0:%s[a]", audioGain, duration)
	case "mix", "":
		base := fmt.Sprintf("[0:a]volume=%s,apad,atrim=0:%s[base]", videoGain, duration)
		if !spec.VideoHasAudio {
			base = fmt.Sprintf("anullsrc=channel_layout=stereo:sample_rate=44100,atrim=0:%s[base]", duration)
		}
		music := fmt.Sprintf("[1:a]volume=%s,apad,atrim=0:%s[music]", audioGain, duration)
		filter = base + ";" + music + ";[base][music]amix=inputs=2:duration=first:normalize=0[a]"
	default:
		return nil, fmt.Errorf("unsupported audio overlay mode %q", spec.Mode)
	}

	args = append(args,
		"-filter_complex", filter,
		"-map", "0:v:0",
		"-map", "[a]",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-t", duration,
		"-movflags", "+faststart",
		spec.OutputPath,
	)
	return args, nil
}

func secondsString(ms int) string {
	return fmt.Sprintf("%.3f", float64(ms)/1000)
}

func volumeGain(v int32) string {
	if v < 0 {
		v = 0
	}
	return fmt.Sprintf("%.2f", float64(v)/100)
}

func mediaExt(key, fallback string) string {
	ext := strings.ToLower(filepath.Ext(key))
	if ext == "" {
		return fallback
	}
	return ext
}

func int4FromPositive(v int) pgtype.Int4 {
	if v <= 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func pickOutputContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		return mediaAudioOverlayOutputType
	}
	return contentType
}
