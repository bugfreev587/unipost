package safefetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestMediaSignaturesProduceVerifiedTemporaryFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mediaType string
		body      []byte
	}{
		{name: "jpeg", mediaType: "image/jpeg", body: append([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}, bytes.Repeat([]byte{0x11}, 64)...)},
		{name: "png", mediaType: "image/png", body: append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x22}, 64)...)},
		{name: "gif", mediaType: "image/gif", body: append([]byte("GIF89a"), bytes.Repeat([]byte{0x33}, 64)...)},
		{name: "webp", mediaType: "image/webp", body: append([]byte("RIFF\x20\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0x44}, 64)...)},
		{name: "mp4", mediaType: "video/mp4", body: append([]byte("\x00\x00\x00\x18ftypisom\x00\x00\x00\x00isommp42"), bytes.Repeat([]byte{0x55}, 64)...)},
		{name: "mov", mediaType: "video/quicktime", body: append([]byte("\x00\x00\x00\x18ftypqt  \x00\x00\x00\x00qt  "), bytes.Repeat([]byte{0x66}, 64)...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()
			body := newTrackedBody(tt.body)
			fetcher := mediaFetcher(tempDir, http.StatusOK, tt.mediaType, body)
			result, err := fetcher.Fetch(context.Background(), "https://cdn.example/media", Policy{
				MaxBytes:          int64(len(tt.body)),
				AllowedMediaTypes: []string{tt.mediaType},
				MaxRedirects:      5,
			})
			if err != nil {
				t.Fatalf("Fetch returned error: %v", err)
			}
			defer result.Close()
			if result.MediaType != tt.mediaType {
				t.Fatalf("MediaType = %q, want %q", result.MediaType, tt.mediaType)
			}
			if result.SizeBytes != int64(len(tt.body)) {
				t.Fatalf("SizeBytes = %d, want %d", result.SizeBytes, len(tt.body))
			}
			sum := sha256.Sum256(tt.body)
			if result.SHA256Hex != hex.EncodeToString(sum[:]) {
				t.Fatalf("SHA256Hex = %q, want %x", result.SHA256Hex, sum)
			}
			info, err := os.Stat(result.Path)
			if err != nil {
				t.Fatalf("stat result: %v", err)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Fatalf("mode = %o, want 600", got)
			}
			file, err := result.Open()
			if err != nil {
				t.Fatalf("open result: %v", err)
			}
			stored, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil || !bytes.Equal(stored, tt.body) {
				t.Fatalf("stored bytes differ: error=%v", err)
			}
			if !body.wasClosed() {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestMediaHTTPStatusMappingDoesNotDiscloseBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    int
		wantKind  ErrorKind
		temporary bool
	}{
		{name: "not found", status: http.StatusNotFound, wantKind: ErrorSourceNotFound},
		{name: "customer rejected", status: http.StatusForbidden, wantKind: ErrorSourceRejected},
		{name: "rate limit", status: http.StatusTooManyRequests, wantKind: ErrorSourceTemporary, temporary: true},
		{name: "provider failure", status: http.StatusBadGateway, wantKind: ErrorSourceTemporary, temporary: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := newTrackedBody([]byte("secret provider response"))
			fetcher := mediaFetcher(t.TempDir(), tt.status, "text/plain", body)
			_, err := fetcher.Fetch(context.Background(), "https://cdn.example/media?token=hidden", Policy{
				MaxBytes:          1024,
				AllowedMediaTypes: []string{"image/png"},
			})
			assertFetchErrorKind(t, err, tt.wantKind)
			var fetchErr *FetchError
			if !errors.As(err, &fetchErr) {
				t.Fatalf("error %T is not *FetchError", err)
			}
			if fetchErr.HTTPStatus != tt.status || fetchErr.Temporary != tt.temporary {
				t.Fatalf("FetchError = %+v, want status=%d temporary=%v", fetchErr, tt.status, tt.temporary)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "token=hidden") {
				t.Fatalf("error disclosed response or URL: %q", err.Error())
			}
			if !body.wasClosed() {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestMediaRejectsEmptyAndTypeConfusedBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		body     []byte
		allowed  []string
		wantKind ErrorKind
	}{
		{name: "empty", header: "image/png", body: nil, allowed: []string{"image/png"}, wantKind: ErrorSourceRejected},
		{name: "html as jpeg", header: "image/jpeg", body: []byte("<!doctype html><html>not an image</html>"), allowed: []string{"image/jpeg"}, wantKind: ErrorUnsupportedMedia},
		{name: "json as png", header: "image/png", body: []byte(`{"error":"not media"}`), allowed: []string{"image/png"}, wantKind: ErrorUnsupportedMedia},
		{name: "svg as png", header: "image/png", body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), allowed: []string{"image/png"}, wantKind: ErrorUnsupportedMedia},
		{name: "executable as jpeg", header: "image/jpeg", body: append([]byte("MZ"), bytes.Repeat([]byte{0}, 64)...), allowed: []string{"image/jpeg"}, wantKind: ErrorUnsupportedMedia},
		{name: "header mismatch", header: "image/png", body: append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0}, 64)...), allowed: []string{"image/jpeg", "image/png"}, wantKind: ErrorUnsupportedMedia},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fetcher := mediaFetcher(t.TempDir(), http.StatusOK, tt.header, newTrackedBody(tt.body))
			_, err := fetcher.Fetch(context.Background(), "https://cdn.example/fake.jpg?real=.png", Policy{
				MaxBytes:          1024,
				AllowedMediaTypes: tt.allowed,
			})
			assertFetchErrorKind(t, err, tt.wantKind)
		})
	}
}

func TestOversizeReadsAtMostLimitPlusOneAndRemovesTemporaryFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	body := newTrackedBody(append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x99}, 4096)...))
	fetcher := mediaFetcher(tempDir, http.StatusOK, "image/png", body)
	_, err := fetcher.Fetch(context.Background(), "https://cdn.example/large.png", Policy{
		MaxBytes:          64,
		AllowedMediaTypes: []string{"image/png"},
	})
	assertFetchErrorKind(t, err, ErrorTooLarge)
	if got := body.bytesRead(); got > 65 {
		t.Fatalf("body read %d bytes, want at most MaxBytes+1", got)
	}
	assertDirectoryEmpty(t, tempDir)
	if !body.wasClosed() {
		t.Fatal("oversized response body was not closed")
	}
}

func TestMediaExactLimitSucceedsAndCleanupIsIdempotent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	data := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x88}, 56)...)
	result, err := mediaFetcher(tempDir, http.StatusOK, "image/png", newTrackedBody(data)).Fetch(
		context.Background(),
		"https://cdn.example/exact.png",
		Policy{MaxBytes: int64(len(data)), AllowedMediaTypes: []string{"image/png"}},
	)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	path := result.Path
	if err := result.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := result.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("result path still exists or stat failed unexpectedly: %v", err)
	}
}

func TestTemporaryFileRemovedWhenReadIsCanceled(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	body := &cancelingBody{data: []byte("\x89PNG\r\n\x1a\npartial")}
	fetcher := mediaFetcher(tempDir, http.StatusOK, "image/png", body)
	_, err := fetcher.Fetch(context.Background(), "https://cdn.example/cancel.png", Policy{
		MaxBytes:          1024,
		AllowedMediaTypes: []string{"image/png"},
	})
	if err == nil {
		t.Fatal("Fetch unexpectedly succeeded")
	}
	assertDirectoryEmpty(t, tempDir)
	if !body.closed {
		t.Fatal("canceled response body was not closed")
	}
}

func mediaFetcher(tempDir string, status int, contentType string, body io.ReadCloser) *fetcher {
	config := DefaultConfig()
	config.TempDir = tempDir
	return &fetcher{
		config: config,
		transportFactory: func(context.Context, *url.URL) (http.RoundTripper, error) {
			return roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				header := make(http.Header)
				if contentType != "" {
					header.Set("Content-Type", contentType)
				}
				return &http.Response{
					StatusCode: status,
					Header:     header,
					Body:       body,
					Request:    request,
				}, nil
			}), nil
		},
	}
}

func assertDirectoryEmpty(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read temp directory: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, filepath.Base(entry.Name()))
		}
		t.Fatalf("temporary files remain: %v", names)
	}
}

type trackedBody struct {
	mu     sync.Mutex
	reader *bytes.Reader
	read   int
	closed bool
}

func newTrackedBody(data []byte) *trackedBody {
	return &trackedBody{reader: bytes.NewReader(data)}
}

func (b *trackedBody) Read(buffer []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	read, err := b.reader.Read(buffer)
	b.read += read
	return read, err
}

func (b *trackedBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *trackedBody) bytesRead() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.read
}

func (b *trackedBody) wasClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

type cancelingBody struct {
	data   []byte
	sent   bool
	closed bool
}

func (b *cancelingBody) Read(buffer []byte) (int, error) {
	if b.sent {
		return 0, context.Canceled
	}
	b.sent = true
	return copy(buffer, b.data), nil
}

func (b *cancelingBody) Close() error {
	b.closed = true
	return nil
}
