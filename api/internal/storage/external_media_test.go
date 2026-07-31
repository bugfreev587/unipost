package storage

import (
	"context"
	"errors"
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
		context.Background(),
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
		if _, err := stageExternalMedia(context.Background(), rawURL, safefetch.Policy{}, fetcher, uploader.put, func(key string) string { return key }); err != nil {
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
		context.Background(),
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
}

func (u *recordingFileUploader) put(_ context.Context, key, path, contentType, _ string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls++
	u.key = key
	u.keys = append(u.keys, key)
	u.contentType = contentType
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return u.err
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
