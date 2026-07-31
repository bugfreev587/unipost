package safefetch

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
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
	absoluteMaxBytes := maximumMediaBytes(policy)
	if absoluteMaxBytes <= 0 {
		return nil, newFetchStatusError(ErrorSourceRejected, response.StatusCode, false)
	}
	if response.ContentLength > absoluteMaxBytes {
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

	prefixLimit := int64(sniffBytes)
	if absoluteMaxBytes < prefixLimit {
		prefixLimit = absoluteMaxBytes + 1
	}
	prefix := make([]byte, int(prefixLimit))
	prefixBytes, readErr := io.ReadFull(response.Body, prefix)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil, mapMediaReadError(ctx, readErr, response.StatusCode)
	}
	if prefixBytes == 0 {
		return nil, newFetchStatusError(ErrorSourceRejected, response.StatusCode, false)
	}
	if int64(prefixBytes) > absoluteMaxBytes {
		return nil, newFetchStatusError(ErrorTooLarge, response.StatusCode, false)
	}

	detected := detectMediaType(prefix[:prefixBytes])
	if detected == "" || !mediaTypeAllowed(detected, policy.AllowedMediaTypes) {
		return nil, newFetchStatusError(ErrorUnsupportedMedia, response.StatusCode, false)
	}
	mediaMaxBytes := mediaTypeMaximumBytes(policy, detected, absoluteMaxBytes)
	if mediaMaxBytes <= 0 {
		return nil, newFetchStatusError(ErrorUnsupportedMedia, response.StatusCode, false)
	}
	if response.ContentLength > mediaMaxBytes || int64(prefixBytes) > mediaMaxBytes {
		return nil, newFetchStatusError(ErrorTooLarge, response.StatusCode, false)
	}
	declared, err := normalizeDeclaredMediaType(response.Header.Get("Content-Type"))
	if err != nil || (declared != "" && declared != "application/octet-stream" && declared != detected) {
		return nil, newFetchStatusError(ErrorUnsupportedMedia, response.StatusCode, false)
	}

	hash := sha256.New()
	destination := io.MultiWriter(file, hash)
	if _, err := destination.Write(prefix[:prefixBytes]); err != nil {
		return nil, mapMediaReadError(ctx, err, response.StatusCode)
	}
	remainingLimit := mediaMaxBytes - int64(prefixBytes) + 1
	remainingBytes, err := io.CopyBuffer(destination, io.LimitReader(response.Body, remainingLimit), make([]byte, 32*1024))
	if err != nil {
		return nil, mapMediaReadError(ctx, err, response.StatusCode)
	}
	written := int64(prefixBytes) + remainingBytes
	if written > mediaMaxBytes {
		return nil, newFetchStatusError(ErrorTooLarge, response.StatusCode, false)
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

func maximumMediaBytes(policy Policy) int64 {
	if policy.MaxBytes > 0 {
		return policy.MaxBytes
	}
	var maximum int64
	for _, limit := range policy.MaxBytesByMediaType {
		if limit > maximum {
			maximum = limit
		}
	}
	return maximum
}

func mediaTypeMaximumBytes(policy Policy, detected string, absoluteMaximum int64) int64 {
	if len(policy.MaxBytesByMediaType) == 0 {
		return absoluteMaximum
	}
	var strictestLimit int64
	for rawMediaType, limit := range policy.MaxBytesByMediaType {
		normalized, err := normalizeDeclaredMediaType(rawMediaType)
		if err != nil || normalized != detected || limit <= 0 {
			continue
		}
		if strictestLimit == 0 || limit < strictestLimit {
			strictestLimit = limit
		}
	}
	if strictestLimit == 0 {
		return 0
	}
	if absoluteMaximum > 0 && strictestLimit > absoluteMaximum {
		return absoluteMaximum
	}
	return strictestLimit
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
		return detectISOBaseMediaType(prefix)
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

func detectISOBaseMediaType(prefix []byte) string {
	if len(prefix) < 16 || string(prefix[4:8]) != "ftyp" {
		return ""
	}
	boxSize := binary.BigEndian.Uint32(prefix[:4])
	if boxSize < 16 || boxSize%4 != 0 || uint64(boxSize) > uint64(len(prefix)) {
		return ""
	}
	box := prefix[:boxSize]
	for offset := 8; offset+4 <= len(box); offset += 4 {
		if isISOBaseImageBrand(string(box[offset : offset+4])) {
			return ""
		}
		if offset == 8 {
			// Skip the four-byte minor version between the major brand and
			// compatible brands.
			offset += 4
		}
	}
	majorBrand := string(prefix[8:12])
	if majorBrand == "qt  " {
		return "video/quicktime"
	}
	if isMP4VideoBrand(majorBrand) {
		return "video/mp4"
	}
	return ""
}

func isISOBaseImageBrand(brand string) bool {
	switch brand {
	case "avif", "avis", "heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs",
		"mif1", "msf1", "miaf", "MiHB", "MiHA", "jpeg", "j2ki", "j2is":
		return true
	default:
		return false
	}
}

func isMP4VideoBrand(brand string) bool {
	switch brand {
	case "isom", "iso2", "iso3", "iso4", "iso5", "iso6", "iso7", "iso8", "iso9",
		"avc1", "dash", "M4V ", "M4VH", "M4VP", "mp41", "mp42", "mp71", "MSNV":
		return true
	default:
		return false
	}
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
