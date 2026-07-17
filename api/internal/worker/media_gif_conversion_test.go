package worker

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

func TestMediaGIFConversionWorkerProcessesAlreadyClaimedJob(t *testing.T) {
	queries := newFakeGIFWorkerQueries()
	store := &fakeGIFWorkerStorage{head: storage.HeadResult{Exists: true, ContentType: "video/mp4", SizeBytes: 4567}, probe: storage.VideoMetadata{Width: 320, Height: 240, DurationMS: 5000}}
	processor := &fakeGIFProcessor{result: gifProcessResult{SizeBytes: 4567, Width: 320, Height: 240, DurationMS: 5000}}
	worker := NewMediaGIFConversionWorker(queries, store).WithProcessor(processor)

	if err := worker.ProcessClaimedJob(context.Background(), gifWorkerJob(1)); err != nil {
		t.Fatal(err)
	}
	if store.downloadLimit != gifMaxCompressedBytes || store.putKey == "" || processor.request.BackgroundColor != "#12ABEF" {
		t.Fatalf("storage/processor calls = limit %d put %q request %#v", store.downloadLimit, store.putKey, processor.request)
	}
	if queries.succeeded.JobID != "job_gif" || !queries.succeeded.OutputMediaID.Valid || queries.succeeded.OutputMediaID.String == "" || !queries.succeeded.CleanupAfterAt.Valid {
		t.Fatalf("completion = %#v", queries.succeeded)
	}
}

func TestMediaGIFConversionWorkerFailsUnsafeGIFWithoutRetry(t *testing.T) {
	queries := newFakeGIFWorkerQueries()
	store := &fakeGIFWorkerStorage{}
	processor := &fakeGIFProcessor{err: &gifProcessingError{Code: gifErrorFrameCountExceeded, Message: "too many frames"}}
	worker := NewMediaGIFConversionWorker(queries, store).WithProcessor(processor)

	err := worker.ProcessClaimedJob(context.Background(), gifWorkerJob(1))
	if err == nil || queries.failed.ErrorCode.String != gifErrorFrameCountExceeded || queries.requeued.JobID != "" || store.putKey != "" {
		t.Fatalf("err=%v failed=%#v requeued=%#v put=%q", err, queries.failed, queries.requeued, store.putKey)
	}
}

func TestMediaGIFConversionWorkerRetriesTransientUploadAndCompensates(t *testing.T) {
	queries := newFakeGIFWorkerQueries()
	store := &fakeGIFWorkerStorage{putErr: errors.New("R2 unavailable")}
	processor := &fakeGIFProcessor{result: gifProcessResult{SizeBytes: 100, Width: 2, Height: 2, DurationMS: 5000}}
	worker := NewMediaGIFConversionWorker(queries, store).WithProcessor(processor)

	err := worker.ProcessClaimedJob(context.Background(), gifWorkerJob(1))
	if err == nil || queries.requeued.JobID != "job_gif" || queries.hardDeleted == "" || store.deletedKey == "" {
		t.Fatalf("err=%v requeued=%#v hard-delete=%q object-delete=%q", err, queries.requeued, queries.hardDeleted, store.deletedKey)
	}
}

func gifWorkerJob(attempts int32) db.MediaProcessingJob {
	return db.MediaProcessingJob{
		ID: "job_gif", WorkspaceID: "ws_1", Kind: mediaGIFConversionKind, Status: "processing", Attempts: attempts,
		InputMediaID: pgtype.Text{String: "med_gif", Valid: true},
		Request:      []byte(`{"gif_media_id":"med_gif","background_color":"#12ABEF","output_profile":"universal_mp4_v1"}`),
	}
}

type fakeGIFWorkerQueries struct {
	media       map[string]db.Media
	succeeded   db.CompleteMediaProcessingJobSucceededParams
	failed      db.CompleteMediaProcessingJobFailedParams
	requeued    db.RequeueMediaProcessingJobParams
	hardDeleted string
}

func newFakeGIFWorkerQueries() *fakeGIFWorkerQueries {
	return &fakeGIFWorkerQueries{media: map[string]db.Media{"med_gif": {
		ID: "med_gif", WorkspaceID: "ws_1", StorageKey: "media/med_gif.gif", ContentType: "image/gif", SizeBytes: 100, Status: "uploaded",
	}}}
}

func (f *fakeGIFWorkerQueries) GetMediaByIDAndWorkspace(_ context.Context, arg db.GetMediaByIDAndWorkspaceParams) (db.Media, error) {
	row, ok := f.media[arg.ID]
	if !ok {
		return db.Media{}, pgx.ErrNoRows
	}
	return row, nil
}
func (f *fakeGIFWorkerQueries) CreateMedia(_ context.Context, arg db.CreateMediaParams) (db.Media, error) {
	row := db.Media{ID: "med_out", WorkspaceID: arg.WorkspaceID, StorageKey: arg.StorageKey, ContentType: arg.ContentType, SizeBytes: arg.SizeBytes, Status: arg.Status}
	f.media[row.ID] = row
	return row, nil
}
func (f *fakeGIFWorkerQueries) UpdateMediaStorageKey(_ context.Context, arg db.UpdateMediaStorageKeyParams) (db.Media, error) {
	row := f.media[arg.ID]
	row.StorageKey = arg.StorageKey
	f.media[arg.ID] = row
	return row, nil
}
func (f *fakeGIFWorkerQueries) MarkMediaUploaded(_ context.Context, arg db.MarkMediaUploadedParams) (db.Media, error) {
	row := f.media[arg.ID]
	row.Status, row.SizeBytes, row.ContentType = "uploaded", arg.SizeBytes, arg.ContentType
	row.Width, row.Height, row.DurationMs = arg.Width, arg.Height, arg.DurationMs
	f.media[arg.ID] = row
	return row, nil
}
func (f *fakeGIFWorkerQueries) HardDeleteMedia(_ context.Context, id string) error {
	f.hardDeleted = id
	delete(f.media, id)
	return nil
}
func (f *fakeGIFWorkerQueries) RequeueMediaProcessingJob(_ context.Context, arg db.RequeueMediaProcessingJobParams) (db.MediaProcessingJob, error) {
	f.requeued = arg
	return db.MediaProcessingJob{ID: arg.JobID}, nil
}
func (f *fakeGIFWorkerQueries) CompleteMediaProcessingJobSucceeded(_ context.Context, arg db.CompleteMediaProcessingJobSucceededParams) (db.MediaProcessingJob, error) {
	f.succeeded = arg
	return db.MediaProcessingJob{ID: arg.JobID, Status: "succeeded"}, nil
}
func (f *fakeGIFWorkerQueries) CompleteMediaProcessingJobFailed(_ context.Context, arg db.CompleteMediaProcessingJobFailedParams) (db.MediaProcessingJob, error) {
	f.failed = arg
	return db.MediaProcessingJob{ID: arg.JobID, Status: "failed"}, nil
}
func (f *fakeGIFWorkerQueries) GetSubscriptionByWorkspace(context.Context, string) (db.Subscription, error) {
	return db.Subscription{}, pgx.ErrNoRows
}

type fakeGIFWorkerStorage struct {
	downloadLimit int64
	putKey        string
	deletedKey    string
	head          storage.HeadResult
	probe         storage.VideoMetadata
	downloadErr   error
	putErr        error
}

func (f *fakeGIFWorkerStorage) DownloadObjectLimited(_ context.Context, _, destination string, limit int64) error {
	f.downloadLimit = limit
	if f.downloadErr != nil {
		return f.downloadErr
	}
	return os.WriteFile(destination, []byte("GIF89a"), 0o600)
}
func (f *fakeGIFWorkerStorage) PutFile(_ context.Context, key, _, _, _ string) error {
	f.putKey = key
	return f.putErr
}
func (f *fakeGIFWorkerStorage) Head(context.Context, string) (storage.HeadResult, error) {
	if !f.head.Exists && f.putErr == nil {
		return storage.HeadResult{Exists: true, ContentType: "video/mp4", SizeBytes: 100}, nil
	}
	return f.head, nil
}
func (f *fakeGIFWorkerStorage) ProbeVideo(context.Context, string) (storage.VideoMetadata, error) {
	return f.probe, nil
}
func (f *fakeGIFWorkerStorage) Delete(_ context.Context, key string) error {
	f.deletedKey = key
	return nil
}

type fakeGIFProcessor struct {
	request gifProcessRequest
	result  gifProcessResult
	err     error
}

func (f *fakeGIFProcessor) Process(_ context.Context, req gifProcessRequest) (gifProcessResult, error) {
	f.request = req
	if f.err == nil {
		_ = os.WriteFile(req.OutputPath, make([]byte, max(1, int(f.result.SizeBytes))), 0o600)
	}
	return f.result, f.err
}
