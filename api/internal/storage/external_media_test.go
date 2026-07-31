package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/safefetch"
)

func TestExternalMediaStagesContentAddressedVerifiedFile(t *testing.T) {
	t.Parallel()

	tempPath := writeVerifiedTemp(t, []byte("verified image bytes"))
	fetcher := &fakeExternalFetcher{result: &safefetch.Result{
		Path:      tempPath,
		MediaType: "image/jpeg",
		SizeBytes: 20,
		SHA256Hex: strings.Repeat("a", 64),
	}}
	uploader := &recordingFileUploader{}
	rawURL := "https://secret.example/private/path/photo.jpg?token=hidden"

	result, err := stageExternalMedia(
		testPublishingLifecycleContext(),
		rawURL,
		safefetch.Policy{MaxBytes: 1024, AllowedMediaTypes: []string{"image/jpeg"}},
		fetcher,
		uploader.put,
		func(key string) string { return "https://media.unipost.example/" + key },
	)
	if err != nil {
		t.Fatalf("stageExternalMedia returned error: %v", err)
	}
	wantKey := "pull/" + strings.Repeat("a", 64) + ".jpg"
	if uploader.key != wantKey {
		t.Fatalf("key = %q, want %q", uploader.key, wantKey)
	}
	if result.PublicURL != "https://media.unipost.example/"+wantKey {
		t.Fatalf("PublicURL = %q", result.PublicURL)
	}
	if result.MediaType != "image/jpeg" || result.SizeBytes != 20 || result.SHA256Hex != strings.Repeat("a", 64) {
		t.Fatalf("result metadata = %+v", result)
	}
	if uploader.contentType != "image/jpeg" {
		t.Fatalf("content type = %q", uploader.contentType)
	}
	if strings.Contains(uploader.key, "secret.example") || strings.Contains(uploader.key, "photo") || strings.Contains(uploader.key, "token") {
		t.Fatalf("object key discloses source URL: %q", uploader.key)
	}
	assertRemoved(t, tempPath)
}

func TestExternalMediaReservesPublishingLifecycleBeforeUpload(t *testing.T) {
	t.Parallel()

	tempPath := writeVerifiedTemp(t, []byte("verified image bytes"))
	fetcher := &fakeExternalFetcher{result: &safefetch.Result{
		Path: tempPath, MediaType: "image/jpeg", SizeBytes: 20, SHA256Hex: strings.Repeat("d", 64),
	}}
	recorder := &recordingPublishingObjectLifecycle{usageIDs: []string{"usage_1"}}
	uploader := &recordingFileUploader{beforePut: func() {
		if len(recorder.reservations) != 1 {
			t.Fatalf("lifecycle reservations before upload = %d, want 1", len(recorder.reservations))
		}
	}}
	ctx := WithPublishingObjectLifecycle(context.Background(), PublishingObjectContext{
		WorkspaceID: "workspace_1",
		PostID:      "post_1",
		Lifecycle:   recorder,
	})

	result, err := stageExternalMedia(
		ctx,
		"https://cdn.example/photo.jpg",
		safefetch.Policy{},
		fetcher,
		uploader.put,
		func(key string) string { return key },
	)
	if err != nil {
		t.Fatalf("stageExternalMedia returned error: %v", err)
	}
	if len(recorder.reservations) != 1 {
		t.Fatalf("lifecycle reservations = %d, want 1", len(recorder.reservations))
	}
	reservation := recorder.reservations[0]
	if reservation.ObjectKey != result.ObjectKey || reservation.WorkspaceID != "workspace_1" || reservation.PostID != "post_1" {
		t.Fatalf("reservation = %#v, result = %#v", reservation, result)
	}
	if len(recorder.abandoned) != 0 {
		t.Fatalf("abandoned reservations = %v, want none", recorder.abandoned)
	}
	if len(recorder.activated) != 1 || recorder.activated[0] != "usage_1" {
		t.Fatalf("activated usages = %v, want usage_1", recorder.activated)
	}
}

func TestExternalMediaUploadFailureReleasesPublishingLifecycle(t *testing.T) {
	t.Parallel()

	tempPath := writeVerifiedTemp(t, []byte("verified image bytes"))
	fetcher := &fakeExternalFetcher{result: &safefetch.Result{
		Path: tempPath, MediaType: "image/jpeg", SizeBytes: 20, SHA256Hex: strings.Repeat("e", 64),
	}}
	recorder := &recordingPublishingObjectLifecycle{usageIDs: []string{"usage_failed"}}
	uploader := &recordingFileUploader{err: errors.New("R2 unavailable")}
	ctx := WithPublishingObjectLifecycle(context.Background(), PublishingObjectContext{
		WorkspaceID: "workspace_1",
		PostID:      "post_1",
		Lifecycle:   recorder,
	})

	_, err := stageExternalMedia(ctx, "https://cdn.example/photo.jpg", safefetch.Policy{}, fetcher, uploader.put, func(key string) string { return key })
	if !errors.Is(err, ErrExternalMediaUpload) {
		t.Fatalf("error = %v, want ErrExternalMediaUpload", err)
	}
	if len(recorder.reservations) != 1 || len(recorder.abandoned) != 1 {
		t.Fatalf("reservations/abandoned = %d/%d, want 1/1", len(recorder.reservations), len(recorder.abandoned))
	}
	if recorder.abandoned[0] != "usage_failed" {
		t.Fatalf("abandoned usage = %q, want usage_failed", recorder.abandoned[0])
	}
}

func TestExternalMediaUploadCancellationAbandonsWithLiveDetachedContext(t *testing.T) {
	t.Parallel()

	tempPath := writeVerifiedTemp(t, []byte("verified image bytes"))
	fetcher := &fakeExternalFetcher{result: &safefetch.Result{
		Path: tempPath, MediaType: "image/jpeg", SizeBytes: 20, SHA256Hex: strings.Repeat("9", 64),
	}}
	recorder := &recordingPublishingObjectLifecycle{usageIDs: []string{"usage_cancelled"}}
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	ctx := WithPublishingObjectLifecycle(requestCtx, PublishingObjectContext{
		WorkspaceID: "workspace_1",
		PostID:      "post_1",
		Lifecycle:   recorder,
	})
	uploader := func(context.Context, string, string, string, string) error {
		cancelRequest()
		return context.Canceled
	}

	_, err := stageExternalMedia(ctx, "https://cdn.example/photo.jpg", safefetch.Policy{}, fetcher, uploader, func(key string) string { return key })
	if !errors.Is(err, ErrExternalMediaUpload) {
		t.Fatalf("error = %v, want ErrExternalMediaUpload", err)
	}
	if len(recorder.abandoned) != 1 || recorder.abandoned[0] != "usage_cancelled" {
		t.Fatalf("abandoned usages = %v, want usage_cancelled", recorder.abandoned)
	}
	if len(recorder.abandonContextErrors) != 1 || recorder.abandonContextErrors[0] != nil {
		t.Fatalf("abandon context errors = %v, want one live detached context", recorder.abandonContextErrors)
	}
}

func TestExternalMediaActivationFailureAbandonsPendingUsage(t *testing.T) {
	t.Parallel()

	tempPath := writeVerifiedTemp(t, []byte("verified image bytes"))
	fetcher := &fakeExternalFetcher{result: &safefetch.Result{
		Path: tempPath, MediaType: "image/jpeg", SizeBytes: 20, SHA256Hex: strings.Repeat("8", 64),
	}}
	recorder := &recordingPublishingObjectLifecycle{
		usageIDs:    []string{"usage_activation_failed"},
		activateErr: errors.New("database unavailable"),
	}
	ctx := WithPublishingObjectLifecycle(context.Background(), PublishingObjectContext{
		WorkspaceID: "workspace_1",
		PostID:      "post_1",
		Lifecycle:   recorder,
	})

	_, err := stageExternalMedia(ctx, "https://cdn.example/photo.jpg", safefetch.Policy{}, fetcher, (&recordingFileUploader{}).put, func(key string) string { return key })
	if !errors.Is(err, ErrExternalMediaLifecycle) {
		t.Fatalf("error = %v, want ErrExternalMediaLifecycle", err)
	}
	if len(recorder.activated) != 1 || recorder.activated[0] != "usage_activation_failed" {
		t.Fatalf("activated usages = %v, want usage_activation_failed", recorder.activated)
	}
	if len(recorder.abandoned) != 1 || recorder.abandoned[0] != "usage_activation_failed" {
		t.Fatalf("abandoned usages = %v, want usage_activation_failed", recorder.abandoned)
	}
}

func TestExternalMediaConcurrentReservationsAbandonOnlyFailedUsage(t *testing.T) {
	t.Parallel()

	sha := strings.Repeat("1", 64)
	recorder := &recordingPublishingObjectLifecycle{usageIDs: []string{"usage_success", "usage_failed"}}
	ctx := WithPublishingObjectLifecycle(context.Background(), PublishingObjectContext{
		WorkspaceID: "workspace_1",
		PostID:      "post_1",
		Lifecycle:   recorder,
	})
	for index, uploadErr := range []error{nil, errors.New("R2 unavailable")} {
		path := writeVerifiedTemp(t, []byte("same verified image bytes"))
		fetcher := &fakeExternalFetcher{result: &safefetch.Result{
			Path: path, MediaType: "image/jpeg", SizeBytes: 25, SHA256Hex: sha,
		}}
		_, err := stageExternalMedia(ctx, "https://cdn.example/photo.jpg", safefetch.Policy{}, fetcher, (&recordingFileUploader{err: uploadErr}).put, func(key string) string { return key })
		if index == 0 && err != nil {
			t.Fatalf("successful reservation returned error: %v", err)
		}
		if index == 1 && !errors.Is(err, ErrExternalMediaUpload) {
			t.Fatalf("failed reservation error = %v, want ErrExternalMediaUpload", err)
		}
	}
	if len(recorder.reservations) != 2 {
		t.Fatalf("reservations = %d, want 2", len(recorder.reservations))
	}
	if len(recorder.abandoned) != 1 || recorder.abandoned[0] != "usage_failed" {
		t.Fatalf("abandoned usages = %v, want only usage_failed", recorder.abandoned)
	}
}

func TestExternalMediaRequiresPublishingLifecycleBeforeUpload(t *testing.T) {
	t.Parallel()

	tempPath := writeVerifiedTemp(t, []byte("verified image bytes"))
	fetcher := &fakeExternalFetcher{result: &safefetch.Result{
		Path: tempPath, MediaType: "image/jpeg", SizeBytes: 20, SHA256Hex: strings.Repeat("f", 64),
	}}
	uploader := &recordingFileUploader{}

	_, err := stageExternalMedia(context.Background(), "https://cdn.example/photo.jpg", safefetch.Policy{}, fetcher, uploader.put, func(key string) string { return key })
	if !errors.Is(err, ErrExternalMediaLifecycle) {
		t.Fatalf("error = %v, want ErrExternalMediaLifecycle", err)
	}
	if uploader.calls != 0 {
		t.Fatalf("storage called %d times without lifecycle reservation", uploader.calls)
	}
}

func TestVerifiedMediaIdenticalContentUsesSameKey(t *testing.T) {
	t.Parallel()

	sha := strings.Repeat("b", 64)
	uploader := &recordingFileUploader{}
	for _, rawURL := range []string{
		"https://one.example/a.jpg?secret=one",
		"https://two.example/completely-different.png?secret=two",
	} {
		path := writeVerifiedTemp(t, []byte("same content"))
		fetcher := &fakeExternalFetcher{result: &safefetch.Result{
			Path: path, MediaType: "image/png", SizeBytes: 12, SHA256Hex: sha,
		}}
		if _, err := stageExternalMedia(testPublishingLifecycleContext(), rawURL, safefetch.Policy{}, fetcher, uploader.put, func(key string) string { return key }); err != nil {
			t.Fatalf("stageExternalMedia(%q): %v", rawURL, err)
		}
	}
	if len(uploader.keys) != 2 || uploader.keys[0] != uploader.keys[1] || uploader.keys[0] != "pull/"+sha+".png" {
		t.Fatalf("keys = %v, want identical content-addressed keys", uploader.keys)
	}
}

func TestExternalMediaFetchFailureSkipsStorage(t *testing.T) {
	t.Parallel()

	fetchErr := &safefetch.FetchError{Kind: safefetch.ErrorSourceNotFound, HTTPStatus: 404}
	fetcher := &fakeExternalFetcher{err: fetchErr}
	uploader := &recordingFileUploader{}
	_, err := stageExternalMedia(context.Background(), "https://secret.example/missing", safefetch.Policy{}, fetcher, uploader.put, func(key string) string { return key })
	if !errors.Is(err, fetchErr) {
		t.Fatalf("error = %v, want original fetch error", err)
	}
	if uploader.calls != 0 {
		t.Fatalf("storage called %d times after fetch failure", uploader.calls)
	}
}

func TestExternalMediaUploadFailureIsTemporaryAndCleansFile(t *testing.T) {
	t.Parallel()

	path := writeVerifiedTemp(t, []byte("verified"))
	fetcher := &fakeExternalFetcher{result: &safefetch.Result{
		Path: path, MediaType: "video/mp4", SizeBytes: 8, SHA256Hex: strings.Repeat("c", 64),
	}}
	uploader := &recordingFileUploader{err: errors.New("R2 unavailable")}
	_, err := stageExternalMedia(
		testPublishingLifecycleContext(),
		"https://secret.example/video.mp4?token=hidden",
		safefetch.Policy{},
		fetcher,
		uploader.put,
		func(key string) string { return key },
	)
	if !errors.Is(err, ErrExternalMediaUpload) {
		t.Fatalf("error = %v, want ErrExternalMediaUpload", err)
	}
	var fetchErr *safefetch.FetchError
	if errors.As(err, &fetchErr) {
		t.Fatalf("storage failure incorrectly presented as fetch failure: %v", err)
	}
	if strings.Contains(err.Error(), "secret.example") || strings.Contains(err.Error(), "token=hidden") {
		t.Fatalf("storage error disclosed source URL: %q", err.Error())
	}
	assertRemoved(t, path)
}

func testPublishingLifecycleContext() context.Context {
	return WithPublishingObjectLifecycle(context.Background(), PublishingObjectContext{
		WorkspaceID: "workspace_test",
		PostID:      "post_test",
		Lifecycle:   &recordingPublishingObjectLifecycle{},
	})
}

func TestExternalMediaNilResultSkipsStorage(t *testing.T) {
	t.Parallel()

	uploader := &recordingFileUploader{}
	_, err := stageExternalMedia(context.Background(), "https://cdn.example/media", safefetch.Policy{}, &fakeExternalFetcher{}, uploader.put, func(key string) string { return key })
	if !errors.Is(err, ErrExternalMediaFetch) {
		t.Fatalf("error = %v, want ErrExternalMediaFetch", err)
	}
	if uploader.calls != 0 {
		t.Fatalf("storage called %d times for nil fetch result", uploader.calls)
	}
}

type fakeExternalFetcher struct {
	mu     sync.Mutex
	result *safefetch.Result
	err    error
	urls   []string
}

func (f *fakeExternalFetcher) Fetch(_ context.Context, rawURL string, _ safefetch.Policy) (*safefetch.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.urls = append(f.urls, rawURL)
	return f.result, f.err
}

type recordingFileUploader struct {
	mu          sync.Mutex
	calls       int
	key         string
	keys        []string
	contentType string
	err         error
	beforePut   func()
}

func (u *recordingFileUploader) put(_ context.Context, key, path, contentType, _ string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls++
	u.key = key
	u.keys = append(u.keys, key)
	u.contentType = contentType
	if u.beforePut != nil {
		u.beforePut()
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return u.err
}

type recordingPublishingObjectLifecycle struct {
	reservations          []PublishingObjectReservation
	usageIDs              []string
	activated             []string
	abandoned             []string
	activateContextErrors []error
	abandonContextErrors  []error
	reserveErr            error
	activateErr           error
	abandonErr            error
}

func (r *recordingPublishingObjectLifecycle) ReservePublishingObject(_ context.Context, reservation PublishingObjectReservation) (string, error) {
	if r.reserveErr != nil {
		return "", r.reserveErr
	}
	r.reservations = append(r.reservations, reservation)
	index := len(r.reservations) - 1
	if index < len(r.usageIDs) {
		return r.usageIDs[index], nil
	}
	return fmt.Sprintf("usage_%d", index+1), nil
}

func (r *recordingPublishingObjectLifecycle) ActivatePublishingObject(ctx context.Context, usageID string) error {
	r.activated = append(r.activated, usageID)
	r.activateContextErrors = append(r.activateContextErrors, ctx.Err())
	return r.activateErr
}

func (r *recordingPublishingObjectLifecycle) AbandonPublishingObject(ctx context.Context, usageID string) error {
	r.abandoned = append(r.abandoned, usageID)
	r.abandonContextErrors = append(r.abandonContextErrors, ctx.Err())
	return r.abandonErr
}

func writeVerifiedTemp(t *testing.T, data []byte) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "verified-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if err := file.Chmod(0o600); err != nil {
		t.Fatalf("chmod temp: %v", err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp: %v", err)
	}
	return file.Name()
}

func assertRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary path %q still exists or stat failed unexpectedly: %v", path, err)
	}
}
