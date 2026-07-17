package worker

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

func TestPreflightGIFReadsBoundedMetadataWithoutDecoding(t *testing.T) {
	body := gifFixture(320, 240, []uint16{25, 75})
	meta, err := preflightGIF(context.Background(), bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("preflight GIF: %v", err)
	}
	if meta.Width != 320 || meta.Height != 240 || meta.Frames != 2 {
		t.Fatalf("metadata = %#v", meta)
	}
	if meta.CycleDuration != time.Second {
		t.Fatalf("cycle duration = %s, want 1s", meta.CycleDuration)
	}
	if meta.DecodedPixels != 320*240*2 {
		t.Fatalf("decoded pixels = %d", meta.DecodedPixels)
	}
}

func TestPreflightGIFAcceptsStaticFrameWithZeroDelay(t *testing.T) {
	body := gifFixture(2, 3, []uint16{0})
	meta, err := preflightGIF(context.Background(), bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("preflight static GIF: %v", err)
	}
	if meta.Frames != 1 || meta.CycleDuration != 0 {
		t.Fatalf("static metadata = %#v", meta)
	}
}

func TestPreflightGIFRejectsSafetyLimits(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		size int64
		code string
	}{
		{name: "compressed bytes", body: gifFixture(1, 1, []uint16{1}), size: gifMaxCompressedBytes + 1, code: gifErrorSizeExceeded},
		{name: "width", body: gifFixture(4097, 1, []uint16{1}), code: gifErrorDimensionsExceeded},
		{name: "height", body: gifFixture(1, 4097, []uint16{1}), code: gifErrorDimensionsExceeded},
		{name: "frames", body: gifFixture(1, 1, make([]uint16, gifMaxFrames+1)), code: gifErrorFrameCountExceeded},
		{name: "duration", body: gifFixture(1, 1, []uint16{6001}), code: gifErrorDurationExceeded},
		{name: "decoded pixels", body: gifFixture(4096, 4096, make([]uint16, 90)), code: gifErrorDecodeBudgetExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.size
			if size == 0 {
				size = int64(len(tt.body))
			}
			_, err := preflightGIF(context.Background(), bytes.NewReader(tt.body), size)
			assertGIFProcessingErrorCode(t, err, tt.code)
		})
	}
}

func TestPreflightGIFAcceptsExactSafetyBoundaries(t *testing.T) {
	body := gifFixture(4096, 4096, make([]uint16, 60))
	meta, err := preflightGIF(context.Background(), bytes.NewReader(body), gifMaxCompressedBytes)
	if err != nil {
		t.Fatalf("exact boundary preflight: %v", err)
	}
	if meta.CycleDuration != 0 || meta.Frames != 60 {
		t.Fatalf("boundary metadata = %#v", meta)
	}
}

func TestPreflightGIFRejectsMalformedAndCancelledInput(t *testing.T) {
	for name, body := range map[string][]byte{
		"not gif":       []byte("not-a-gif"),
		"truncated lsd": []byte("GIF89a\x01"),
		"missing trailer": func() []byte {
			body := gifFixture(1, 1, []uint16{1})
			return body[:len(body)-1]
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := preflightGIF(context.Background(), bytes.NewReader(body), int64(len(body)))
			assertGIFProcessingErrorCode(t, err, gifErrorProbeFailed)
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := gifFixture(1, 1, []uint16{1})
	_, err := preflightGIF(ctx, bytes.NewReader(body), int64(len(body)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preflight error = %v", err)
	}
}

func assertGIFProcessingErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	var processingErr *gifProcessingError
	if !errors.As(err, &processingErr) {
		t.Fatalf("error = %v, want gifProcessingError %q", err, want)
	}
	if processingErr.Code != want {
		t.Fatalf("error code = %q, want %q", processingErr.Code, want)
	}
}

func gifFixture(width, height uint16, delays []uint16) []byte {
	var body bytes.Buffer
	body.WriteString("GIF89a")
	_ = binary.Write(&body, binary.LittleEndian, width)
	_ = binary.Write(&body, binary.LittleEndian, height)
	body.Write([]byte{0x00, 0x00, 0x00}) // no global color table
	for _, delay := range delays {
		body.Write([]byte{0x21, 0xF9, 0x04, 0x00})
		_ = binary.Write(&body, binary.LittleEndian, delay)
		body.Write([]byte{0x00, 0x00})
		body.WriteByte(0x2C)
		body.Write(make([]byte, 4))
		_ = binary.Write(&body, binary.LittleEndian, width)
		_ = binary.Write(&body, binary.LittleEndian, height)
		body.WriteByte(0x00)
		body.Write([]byte{0x02, 0x01, 0x00, 0x00})
	}
	body.WriteByte(0x3B)
	return body.Bytes()
}
