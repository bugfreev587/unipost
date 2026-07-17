package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	mediaGIFConversionKind       = "gif_to_mp4"
	mediaGIFConversionOutputType = "video/mp4"
	mediaGIFOutputProfile        = "universal_mp4_v1"
)

type mediaGIFConversionWorkerQueries interface {
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

type mediaGIFConversionWorkerStorage interface {
	DownloadObjectLimited(context.Context, string, string, int64) error
	PutFile(context.Context, string, string, string, string) error
	Head(context.Context, string) (storage.HeadResult, error)
	ProbeVideo(context.Context, string) (storage.VideoMetadata, error)
	Delete(context.Context, string) error
}

type MediaGIFConversionWorker struct {
	queries   mediaGIFConversionWorkerQueries
	storage   mediaGIFConversionWorkerStorage
	processor gifProcessor
}

func NewMediaGIFConversionWorker(queries mediaGIFConversionWorkerQueries, store mediaGIFConversionWorkerStorage) *MediaGIFConversionWorker {
	return &MediaGIFConversionWorker{queries: queries, storage: store, processor: newFFmpegGIFProcessor()}
}

func (w *MediaGIFConversionWorker) WithProcessor(processor gifProcessor) *MediaGIFConversionWorker {
	w.processor = processor
	return w
}

type gifConversionJobRequest struct {
	GIFMediaID      string `json:"gif_media_id"`
	BackgroundColor string `json:"background_color"`
	OutputProfile   string `json:"output_profile"`
}

// ProcessClaimedJob accepts only a job whose status/attempt was already
// transitioned by the shared coordinator. It never claims another kind.
func (w *MediaGIFConversionWorker) ProcessClaimedJob(ctx context.Context, job db.MediaProcessingJob) error {
	if w == nil || w.queries == nil || w.storage == nil || w.processor == nil {
		return fmt.Errorf("GIF conversion worker is not configured")
	}
	if job.Kind != mediaGIFConversionKind {
		return fmt.Errorf("unsupported media processing job kind %q", job.Kind)
	}
	if !job.InputMediaID.Valid || strings.TrimSpace(job.InputMediaID.String) == "" {
		return w.failJob(ctx, job, "invalid_media_processing_job", "GIF input media id is missing", false)
	}
	var request gifConversionJobRequest
	if err := json.Unmarshal(job.Request, &request); err != nil || request.GIFMediaID != job.InputMediaID.String || !gifBackgroundPattern.MatchString(request.BackgroundColor) || request.OutputProfile != mediaGIFOutputProfile {
		return w.failJob(ctx, job, "invalid_media_processing_job", "GIF conversion request is invalid", false)
	}
	input, err := w.queries.GetMediaByIDAndWorkspace(ctx, db.GetMediaByIDAndWorkspaceParams{ID: job.InputMediaID.String, WorkspaceID: job.WorkspaceID})
	if err != nil || input.Status == "deleted" {
		return w.failJob(ctx, job, "input_media_unavailable", "GIF input media is unavailable", false)
	}
	if strings.ToLower(strings.TrimSpace(strings.SplitN(input.ContentType, ";", 2)[0])) != "image/gif" {
		return w.failJob(ctx, job, "gif_media_required", "GIF input media is not image/gif", false)
	}

	tmpDir, err := os.MkdirTemp("", "unipost-gif-conversion-*")
	if err != nil {
		return w.failJob(ctx, job, "gif_conversion_failed", "GIF processing workspace could not be created", true)
	}
	defer os.RemoveAll(tmpDir)
	inputPath := filepath.Join(tmpDir, "input.gif")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := w.storage.DownloadObjectLimited(ctx, input.StorageKey, inputPath, gifMaxCompressedBytes); err != nil {
		if errors.Is(err, storage.ErrObjectTooLarge) {
			return w.failJob(ctx, job, gifErrorSizeExceeded, "GIF exceeds the 50 MB compressed size limit", false)
		}
		return w.failJob(ctx, job, "input_media_unavailable", "GIF input object could not be downloaded", true)
	}
	result, err := w.processor.Process(ctx, gifProcessRequest{InputPath: inputPath, OutputPath: outputPath, BackgroundColor: request.BackgroundColor})
	if err != nil {
		code, message, retryable := classifyGIFProcessingFailure(err)
		return w.failJob(ctx, job, code, message, retryable)
	}
	output, err := w.createOutputMedia(ctx, job, outputPath, result)
	if err != nil {
		return w.failJob(ctx, job, "output_upload_failed", "Converted MP4 could not be stored", true)
	}
	cleanupAfter, err := w.processingCleanupDeadline(ctx, job.WorkspaceID, "published")
	if err != nil {
		w.compensateOutput(ctx, output)
		return w.failJob(ctx, job, "media_processing_completion_failed", "GIF conversion retention could not be assigned", true)
	}
	if _, err := w.queries.CompleteMediaProcessingJobSucceeded(ctx, db.CompleteMediaProcessingJobSucceededParams{
		JobID: job.ID, OutputMediaID: pgtype.Text{String: output.ID, Valid: true}, CleanupAfterAt: cleanupAfter,
	}); err != nil {
		w.compensateOutput(ctx, output)
		return w.failJob(ctx, job, "media_processing_completion_failed", "GIF conversion could not be completed", true)
	}
	slog.Info("media GIF conversion succeeded", "job_id", job.ID, "workspace_id", job.WorkspaceID, "bytes", result.SizeBytes, "width", result.Width, "height", result.Height, "duration_ms", result.DurationMS)
	return nil
}

func (w *MediaGIFConversionWorker) createOutputMedia(ctx context.Context, job db.MediaProcessingJob, outputPath string, result gifProcessResult) (db.Media, error) {
	row, err := w.queries.CreateMedia(ctx, db.CreateMediaParams{
		WorkspaceID: job.WorkspaceID, StorageKey: "placeholder/" + uuid.NewString(), ContentType: mediaGIFConversionOutputType,
		SizeBytes: result.SizeBytes, Status: "pending", ContentHash: pgtype.Text{},
	})
	if err != nil {
		return db.Media{}, err
	}
	finalKey := storage.MediaKey(row.ID, ".mp4")
	row, err = w.queries.UpdateMediaStorageKey(ctx, db.UpdateMediaStorageKeyParams{ID: row.ID, StorageKey: finalKey})
	if err != nil {
		_ = w.queries.HardDeleteMedia(ctx, row.ID)
		return db.Media{}, err
	}
	if err := w.storage.PutFile(ctx, finalKey, outputPath, mediaGIFConversionOutputType, "public, max-age=31536000, immutable"); err != nil {
		_ = w.storage.Delete(ctx, finalKey)
		_ = w.queries.HardDeleteMedia(ctx, row.ID)
		return db.Media{}, err
	}
	head, err := w.storage.Head(ctx, finalKey)
	if err != nil || !head.Exists || head.SizeBytes <= 0 || head.SizeBytes > gifOutputHardCapBytes {
		w.compensateOutput(ctx, row)
		return db.Media{}, fmt.Errorf("uploaded output could not be verified")
	}
	meta, err := w.storage.ProbeVideo(ctx, finalKey)
	if err != nil {
		w.compensateOutput(ctx, row)
		return db.Media{}, err
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
		ID: row.ID, SizeBytes: head.SizeBytes, ContentType: mediaGIFConversionOutputType,
		Width: int4FromPositive(meta.Width), Height: int4FromPositive(meta.Height), DurationMs: int4FromPositive(meta.DurationMS),
	})
	if err != nil {
		w.compensateOutput(ctx, row)
		return db.Media{}, err
	}
	return marked, nil
}

func (w *MediaGIFConversionWorker) compensateOutput(ctx context.Context, output db.Media) {
	if strings.HasPrefix(output.StorageKey, storage.MediaPrefix) {
		if err := w.storage.Delete(ctx, output.StorageKey); err != nil {
			slog.Error("media GIF conversion output compensation object delete failed", "media_id", output.ID, "error", err)
		}
	}
	if err := w.queries.HardDeleteMedia(ctx, output.ID); err != nil {
		slog.Error("media GIF conversion output compensation row delete failed", "media_id", output.ID, "error", err)
	}
}

func classifyGIFProcessingFailure(err error) (code, message string, retryable bool) {
	var processingErr *gifProcessingError
	if !errors.As(err, &processingErr) {
		return "gif_conversion_failed", "GIF conversion failed", true
	}
	switch processingErr.Code {
	case gifErrorProcessingFailed, gifErrorOutputInvalid:
		return "gif_conversion_failed", "GIF could not be rendered as a valid universal MP4", false
	case gifErrorProcessingTimeout:
		return "processing_timeout", "GIF conversion exceeded the five minute processing limit", true
	default:
		return processingErr.Code, processingErr.Message, processingErr.Retryable
	}
}

func (w *MediaGIFConversionWorker) failJob(ctx context.Context, job db.MediaProcessingJob, code, message string, retryable bool) error {
	if retryable && job.Attempts < 3 {
		_, err := w.queries.RequeueMediaProcessingJob(ctx, db.RequeueMediaProcessingJobParams{
			ErrorCode: pgtype.Text{String: code, Valid: true}, ErrorMessage: pgtype.Text{String: message, Valid: true}, JobID: job.ID,
		})
		if err != nil {
			return fmt.Errorf("%s: requeue retryable failure: %w", message, err)
		}
		return errors.New(message)
	}
	if retryable {
		message = "Processing attempts exhausted: " + message
	}
	cleanupAfter, deadlineErr := w.processingCleanupDeadline(ctx, job.WorkspaceID, "failed")
	if deadlineErr != nil {
		return fmt.Errorf("%s: calculate media processing failure retention: %w", message, deadlineErr)
	}
	_, err := w.queries.CompleteMediaProcessingJobFailed(ctx, db.CompleteMediaProcessingJobFailedParams{
		JobID: job.ID, ErrorCode: pgtype.Text{String: code, Valid: true}, ErrorMessage: pgtype.Text{String: message, Valid: true}, CleanupAfterAt: cleanupAfter,
	})
	if err != nil {
		return fmt.Errorf("%s: complete terminal failure: %w", message, err)
	}
	return errors.New(message)
}

func (w *MediaGIFConversionWorker) processingCleanupDeadline(ctx context.Context, workspaceID, terminalStatus string) (pgtype.Timestamptz, error) {
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
