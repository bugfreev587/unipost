package mediaprocessing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

const admissionLockNamespace = "media-processing-admission:"

type AdmissionCode string

const (
	AdmissionAccepted           AdmissionCode = "accepted"
	AdmissionIdempotentReplay   AdmissionCode = "idempotent_replay"
	AdmissionIdempotentConflict AdmissionCode = "idempotency_conflict"
	AdmissionCapacityExceeded   AdmissionCode = "media_processing_capacity_exceeded"
	AdmissionGIFRateExceeded    AdmissionCode = "gif_conversion_rate_limit_exceeded"
)

type AdmissionDecision struct {
	Code       AdmissionCode
	RetryAfter time.Duration
	ResetAt    time.Time
}

type GIFAdmissionRequest struct {
	WorkspaceID    string
	InputMediaID   string
	RequestJSON    []byte
	RequestHash    string
	IdempotencyKey string
	Now            time.Time
}

type AdmissionResult struct {
	Decision AdmissionDecision
	Job      db.MediaProcessingJob
}

type GIFAdmitter interface {
	AdmitGIF(context.Context, GIFAdmissionRequest) (AdmissionResult, error)
}

type AudioOverlayAdmissionRequest struct {
	WorkspaceID       string
	InputVideoMediaID string
	InputAudioMediaID string
	Mode              string
	Fit               string
	VideoVolume       int32
	AudioVolume       int32
	AudioStartMS      int32
	RequestJSON       []byte
	RequestHash       string
	IdempotencyKey    string
	Now               time.Time
}

type AudioOverlayAdmitter interface {
	AdmitAudioOverlay(context.Context, AudioOverlayAdmissionRequest) (AdmissionResult, error)
}

type EnterpriseOverrideResolver interface {
	MediaProcessingLimits(context.Context, string) (*Limits, error)
}

type PostgresAdmitter struct {
	pool             *pgxpool.Pool
	overrideResolver EnterpriseOverrideResolver
}

func NewPostgresAdmitter(pool *pgxpool.Pool) *PostgresAdmitter {
	return &PostgresAdmitter{pool: pool}
}

func (a *PostgresAdmitter) WithEnterpriseOverrideResolver(resolver EnterpriseOverrideResolver) *PostgresAdmitter {
	a.overrideResolver = resolver
	return a
}

func EvaluateAdmission(limits Limits, activeJobs, rollingGIF int64, oldestGIF, now time.Time) AdmissionDecision {
	if activeJobs >= int64(limits.ActiveJobs) {
		return AdmissionDecision{Code: AdmissionCapacityExceeded, RetryAfter: 30 * time.Second}
	}
	if rollingGIF >= int64(limits.GIFConversions24H) {
		resetAt := oldestGIF.Add(24 * time.Hour)
		retryAfter := time.Until(resetAt)
		if !now.IsZero() {
			retryAfter = resetAt.Sub(now)
		}
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return AdmissionDecision{Code: AdmissionGIFRateExceeded, RetryAfter: retryAfter, ResetAt: resetAt}
	}
	return AdmissionDecision{Code: AdmissionAccepted}
}

func (a *PostgresAdmitter) AdmitGIF(ctx context.Context, req GIFAdmissionRequest) (result AdmissionResult, err error) {
	if a == nil || a.pool == nil {
		return AdmissionResult{}, fmt.Errorf("media processing admission is not configured")
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdmissionResult{}, fmt.Errorf("begin media processing admission: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, admissionLockNamespace+req.WorkspaceID); err != nil {
		return AdmissionResult{}, fmt.Errorf("lock media processing admission: %w", err)
	}
	queries := db.New(tx)

	if req.IdempotencyKey != "" {
		existing, lookupErr := queries.GetMediaProcessingJobByIdempotencyKey(ctx, db.GetMediaProcessingJobByIdempotencyKeyParams{
			WorkspaceID:    req.WorkspaceID,
			IdempotencyKey: pgtype.Text{String: req.IdempotencyKey, Valid: true},
		})
		if lookupErr == nil {
			decision := AdmissionDecision{Code: AdmissionIdempotentConflict}
			if existing.RequestHash.Valid && existing.RequestHash.String == req.RequestHash {
				decision.Code = AdmissionIdempotentReplay
			}
			if err = tx.Commit(ctx); err != nil {
				return AdmissionResult{}, fmt.Errorf("commit media processing replay: %w", err)
			}
			return AdmissionResult{Decision: decision, Job: existing}, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return AdmissionResult{}, fmt.Errorf("lookup media processing idempotency: %w", lookupErr)
		}
	}

	planID := "free"
	if subscription, subscriptionErr := queries.GetSubscriptionByWorkspace(ctx, req.WorkspaceID); subscriptionErr == nil {
		planID = subscription.PlanID
	} else if !errors.Is(subscriptionErr, pgx.ErrNoRows) {
		return AdmissionResult{}, fmt.Errorf("load media processing plan: %w", subscriptionErr)
	}
	var enterpriseOverride *Limits
	if planID == "enterprise" && a.overrideResolver != nil {
		enterpriseOverride, err = a.overrideResolver.MediaProcessingLimits(ctx, req.WorkspaceID)
		if err != nil {
			return AdmissionResult{}, fmt.Errorf("load enterprise media processing limits: %w", err)
		}
	}
	limits := LimitsForPlan(planID, enterpriseOverride)
	active, err := queries.CountActiveMediaProcessingJobsByWorkspace(ctx, req.WorkspaceID)
	if err != nil {
		return AdmissionResult{}, fmt.Errorf("count active media processing jobs: %w", err)
	}
	since := pgtype.Timestamptz{Time: req.Now.Add(-24 * time.Hour), Valid: true}
	rolling, err := queries.CountGIFConversionsSince(ctx, db.CountGIFConversionsSinceParams{WorkspaceID: req.WorkspaceID, CreatedSince: since})
	if err != nil {
		return AdmissionResult{}, fmt.Errorf("count rolling GIF conversions: %w", err)
	}
	var oldest time.Time
	if rolling >= int64(limits.GIFConversions24H) {
		oldestValue, oldestErr := queries.OldestGIFConversionCreatedSince(ctx, db.OldestGIFConversionCreatedSinceParams{WorkspaceID: req.WorkspaceID, CreatedSince: since})
		if oldestErr != nil {
			return AdmissionResult{}, fmt.Errorf("load rolling GIF reset: %w", oldestErr)
		}
		if oldestValue.Valid {
			oldest = oldestValue.Time
		}
	}
	decision := EvaluateAdmission(limits, active, rolling, oldest, req.Now)
	if decision.Code != AdmissionAccepted {
		if err = tx.Commit(ctx); err != nil {
			return AdmissionResult{}, fmt.Errorf("commit media processing rejection: %w", err)
		}
		return AdmissionResult{Decision: decision}, nil
	}

	params := db.CreateGIFMediaProcessingJobParams{
		WorkspaceID:  req.WorkspaceID,
		InputMediaID: pgtype.Text{String: req.InputMediaID, Valid: true},
		RequestJson:  req.RequestJSON,
		RequestHash:  pgtype.Text{String: req.RequestHash, Valid: true},
	}
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = pgtype.Text{String: req.IdempotencyKey, Valid: true}
	}
	created, err := queries.CreateGIFMediaProcessingJob(ctx, params)
	if err != nil {
		return AdmissionResult{}, fmt.Errorf("create GIF media processing job: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return AdmissionResult{}, fmt.Errorf("commit GIF media processing job: %w", err)
	}
	return AdmissionResult{Decision: decision, Job: mediaProcessingJobFromGIFCreateRow(created)}, nil
}

func (a *PostgresAdmitter) AdmitAudioOverlay(ctx context.Context, req AudioOverlayAdmissionRequest) (result AdmissionResult, err error) {
	if a == nil || a.pool == nil {
		return AdmissionResult{}, fmt.Errorf("media processing admission is not configured")
	}
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdmissionResult{}, fmt.Errorf("begin media processing admission: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, admissionLockNamespace+req.WorkspaceID); err != nil {
		return AdmissionResult{}, fmt.Errorf("lock media processing admission: %w", err)
	}
	queries := db.New(tx)
	if req.IdempotencyKey != "" {
		existing, lookupErr := queries.GetMediaProcessingJobByIdempotencyKey(ctx, db.GetMediaProcessingJobByIdempotencyKeyParams{
			WorkspaceID:    req.WorkspaceID,
			IdempotencyKey: pgtype.Text{String: req.IdempotencyKey, Valid: true},
		})
		if lookupErr == nil {
			decision := AdmissionDecision{Code: AdmissionIdempotentConflict}
			if existing.RequestHash.Valid && existing.RequestHash.String == req.RequestHash {
				decision.Code = AdmissionIdempotentReplay
			}
			if err = tx.Commit(ctx); err != nil {
				return AdmissionResult{}, fmt.Errorf("commit media processing replay: %w", err)
			}
			return AdmissionResult{Decision: decision, Job: existing}, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return AdmissionResult{}, fmt.Errorf("lookup media processing idempotency: %w", lookupErr)
		}
	}
	planID := "free"
	if subscription, subscriptionErr := queries.GetSubscriptionByWorkspace(ctx, req.WorkspaceID); subscriptionErr == nil {
		planID = subscription.PlanID
	} else if !errors.Is(subscriptionErr, pgx.ErrNoRows) {
		return AdmissionResult{}, fmt.Errorf("load media processing plan: %w", subscriptionErr)
	}
	var enterpriseOverride *Limits
	if planID == "enterprise" && a.overrideResolver != nil {
		enterpriseOverride, err = a.overrideResolver.MediaProcessingLimits(ctx, req.WorkspaceID)
		if err != nil {
			return AdmissionResult{}, fmt.Errorf("load enterprise media processing limits: %w", err)
		}
	}
	limits := LimitsForPlan(planID, enterpriseOverride)
	active, err := queries.CountActiveMediaProcessingJobsByWorkspace(ctx, req.WorkspaceID)
	if err != nil {
		return AdmissionResult{}, fmt.Errorf("count active media processing jobs: %w", err)
	}
	decision := EvaluateAdmission(limits, active, 0, time.Time{}, req.Now)
	if decision.Code != AdmissionAccepted {
		if err = tx.Commit(ctx); err != nil {
			return AdmissionResult{}, fmt.Errorf("commit media processing rejection: %w", err)
		}
		return AdmissionResult{Decision: decision}, nil
	}
	params := db.CreateAudioOverlayMediaProcessingJobParams{
		WorkspaceID:       req.WorkspaceID,
		InputVideoMediaID: pgtype.Text{String: req.InputVideoMediaID, Valid: true},
		InputAudioMediaID: pgtype.Text{String: req.InputAudioMediaID, Valid: true},
		Mode:              req.Mode, Fit: req.Fit, VideoVolume: req.VideoVolume, AudioVolume: req.AudioVolume,
		AudioStartMs: req.AudioStartMS, RequestJson: req.RequestJSON,
		RequestHash: pgtype.Text{String: req.RequestHash, Valid: true},
	}
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = pgtype.Text{String: req.IdempotencyKey, Valid: true}
	}
	created, err := queries.CreateAudioOverlayMediaProcessingJob(ctx, params)
	if err != nil {
		return AdmissionResult{}, fmt.Errorf("create Audio Overlay media processing job: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return AdmissionResult{}, fmt.Errorf("commit Audio Overlay media processing job: %w", err)
	}
	return AdmissionResult{Decision: decision, Job: mediaProcessingJobFromAudioCreateRow(created)}, nil
}

func mediaProcessingJobFromGIFCreateRow(row db.CreateGIFMediaProcessingJobRow) db.MediaProcessingJob {
	return db.MediaProcessingJob{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Kind: row.Kind, Status: row.Status,
		InputVideoMediaID: row.InputVideoMediaID, InputAudioMediaID: row.InputAudioMediaID,
		OutputMediaID: row.OutputMediaID, Mode: row.Mode, Fit: row.Fit,
		VideoVolume: row.VideoVolume, AudioVolume: row.AudioVolume, AudioStartMs: row.AudioStartMs,
		Request: row.Request, IdempotencyKey: row.IdempotencyKey, RequestHash: row.RequestHash,
		ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, Retryable: row.Retryable,
		Attempts: row.Attempts, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, InputMediaID: row.InputMediaID,
		NextAttemptAt: row.NextAttemptAt,
	}
}

func mediaProcessingJobFromAudioCreateRow(row db.CreateAudioOverlayMediaProcessingJobRow) db.MediaProcessingJob {
	return db.MediaProcessingJob{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Kind: row.Kind, Status: row.Status,
		InputVideoMediaID: row.InputVideoMediaID, InputAudioMediaID: row.InputAudioMediaID,
		OutputMediaID: row.OutputMediaID, Mode: row.Mode, Fit: row.Fit,
		VideoVolume: row.VideoVolume, AudioVolume: row.AudioVolume, AudioStartMs: row.AudioStartMs,
		Request: row.Request, IdempotencyKey: row.IdempotencyKey, RequestHash: row.RequestHash,
		ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, Retryable: row.Retryable,
		Attempts: row.Attempts, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, InputMediaID: row.InputMediaID,
		NextAttemptAt: row.NextAttemptAt,
	}
}
