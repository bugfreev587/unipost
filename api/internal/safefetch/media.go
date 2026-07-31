package safefetch

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const sniffBytes = 512

func storeVerifiedMedia(ctx context.Context, response *http.Response, policy Policy, tempDir string) (*Result, error) {
	if response == nil {
		return nil, newFetchStatusError(ErrorSourceTemporary, 0, true)
	}
	if err := validateMediaResponseStatus(response.StatusCode); err != nil {
		return nil, err
	}
	if response.Body == nil {
		return nil, newFetchStatusError(ErrorSourceRejected, response.StatusCode, false)
	}
	if policy.MaxBytes <= 0 {
		return nil, newFetchStatusError(ErrorSourceRejected, response.StatusCode, false)
	}
	if response.ContentLength > policy.MaxBytes {
		return nil, newFetchStatusError(ErrorTooLarge, response.StatusCode, false)
	}

	file, path, err := createPrivateTemp(tempDir)
	if err != nil {
		return nil, newFetchStatusError(ErrorSourceTemporary, response.StatusCode, true)
	}
	complete := false
	defer func() {
		if !complete {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()

	hash := sha256.New()
	prefix := &prefixWriter{limit: sniffBytes}
	limited := io.LimitReader(response.Body, policy.MaxBytes+1)
	written, err := io.CopyBuffer(io.MultiWriter(file, hash, prefix), limited, make([]byte, 32*1024))
	if err != nil {
		return nil, mapMediaReadError(ctx, err, response.StatusCode)
	}
	if written > policy.MaxBytes {
		return nil, newFetchStatusError(ErrorTooLarge, response.StatusCode, false)
	}
	if written == 0 {
		return nil, newFetchStatusError(ErrorSourceRejected, response.StatusCode, false)
	}

	detected := detectMediaType(prefix.bytes)
	if detected == "" || !mediaTypeAllowed(detected, policy.AllowedMediaTypes) {
		return nil, newFetchStatusError(ErrorUnsupportedMedia, response.StatusCode, false)
	}
	declared, err := normalizeDeclaredMediaType(response.Header.Get("Content-Type"))
	if err != nil || (declared != "" && declared != "application/octet-stream" && declared != detected) {
		return nil, newFetchStatusError(ErrorUnsupportedMedia, response.StatusCode, false)
	}
	if err := file.Sync(); err != nil {
		return nil, newFetchStatusError(ErrorSourceTemporary, response.StatusCode, true)
	}
	if err := file.Close(); err != nil {
		return nil, newFetchStatusError(ErrorSourceTemporary, response.StatusCode, true)
	}
	complete = true
	return &Result{
		Path:      path,
		MediaType: detected,
		SizeBytes: written,
		SHA256Hex: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func validateMediaResponseStatus(status int) error {
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusNotFound:
		return newFetchStatusError(ErrorSourceNotFound, status, false)
	case status == http.StatusTooManyRequests || status >= http.StatusInternalServerError:
		return newFetchStatusError(ErrorSourceTemporary, status, true)
	default:
		return newFetchStatusError(ErrorSourceRejected, status, false)
	}
}

func createPrivateTemp(directory string) (*os.File, string, error) {
	if directory == "" {
		directory = os.TempDir()
	}
	for range 10 {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return nil, "", err
		}
		path := filepath.Join(directory, "unipost-remote-media-"+hex.EncodeToString(random))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return file, path, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("temporary file collision limit reached")
}

func detectMediaType(prefix []byte) string {
	if len(prefix) >= 12 && string(prefix[0:4]) == "RIFF" && string(prefix[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(prefix) >= 12 && string(prefix[4:8]) == "ftyp" {
		if string(prefix[8:12]) == "qt  " {
			return "video/quicktime"
		}
		return "video/mp4"
	}
	if len(prefix) >= 4 && prefix[0] == 0x1a && prefix[1] == 0x45 && prefix[2] == 0xdf && prefix[3] == 0xa3 {
		return "video/webm"
	}
	detected := http.DetectContentType(prefix)
	normalized, err := normalizeDeclaredMediaType(detected)
	if err != nil {
		return ""
	}
	return normalized
}

func normalizeDeclaredMediaType(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return "", err
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch mediaType {
	case "image/jpg":
		return "image/jpeg", nil
	case "video/x-quicktime":
		return "video/quicktime", nil
	default:
		return mediaType, nil
	}
}

func mediaTypeAllowed(detected string, allowed []string) bool {
	for _, candidate := range allowed {
		normalized, err := normalizeDeclaredMediaType(candidate)
		if err == nil && normalized == detected {
			return true
		}
	}
	return false
}

func mapMediaReadError(ctx context.Context, err error, status int) error {
	var fetchErr *FetchError
	if errors.As(err, &fetchErr) {
		return fetchErr
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return newFetchStatusError(ErrorTimeout, status, true)
	}
	return newFetchStatusError(ErrorSourceTemporary, status, true)
}

type prefixWriter struct {
	bytes []byte
	limit int
}

func (w *prefixWriter) Write(buffer []byte) (int, error) {
	remaining := w.limit - len(w.bytes)
	if remaining > 0 {
		if remaining > len(buffer) {
			remaining = len(buffer)
		}
		w.bytes = append(w.bytes, buffer[:remaining]...)
	}
	return len(buffer), nil
}
