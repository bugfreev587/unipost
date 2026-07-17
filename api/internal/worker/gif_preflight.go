package worker

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

const (
	gifMaxCompressedBytes int64 = 50 * 1024 * 1024
	gifMaxDimension             = 4096
	gifMaxFrames                = 2000
	gifMaxDecodedPixels   int64 = 1_500_000_000
	gifMaxCycleDuration         = 60 * time.Second

	gifErrorSizeExceeded         = "gif_size_exceeded"
	gifErrorDimensionsExceeded   = "gif_dimensions_exceeded"
	gifErrorFrameCountExceeded   = "gif_frame_count_exceeded"
	gifErrorDecodeBudgetExceeded = "gif_decode_budget_exceeded"
	gifErrorDurationExceeded     = "gif_duration_exceeded"
	gifErrorProbeFailed          = "gif_probe_failed"
	gifErrorDecodeFailed         = "gif_decode_failed"
	gifErrorOutputInvalid        = "gif_output_invalid"
	gifErrorOutputSizeExceeded   = "output_size_exceeded"
	gifErrorProcessingFailed     = "gif_processing_failed"
	gifErrorProcessingTimeout    = "gif_processing_timeout"
)

type gifMetadata struct {
	Width         int
	Height        int
	Frames        int
	CycleDuration time.Duration
	DecodedPixels int64
}

type gifProcessingError struct {
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *gifProcessingError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *gifProcessingError) Unwrap() error { return e.Cause }

func preflightGIF(ctx context.Context, source io.Reader, compressedBytes int64) (gifMetadata, error) {
	if err := ctx.Err(); err != nil {
		return gifMetadata{}, err
	}
	if compressedBytes < 0 || compressedBytes > gifMaxCompressedBytes {
		return gifMetadata{}, gifPreflightError(gifErrorSizeExceeded, "GIF exceeds the 50 MB compressed size limit", nil)
	}

	reader := &gifMetadataReader{ctx: ctx, reader: io.LimitReader(source, gifMaxCompressedBytes+1)}
	header := make([]byte, 6)
	if err := reader.readFull(header); err != nil {
		return gifMetadata{}, gifMalformed(err)
	}
	if string(header) != "GIF87a" && string(header) != "GIF89a" {
		return gifMetadata{}, gifMalformed(fmt.Errorf("invalid GIF signature"))
	}

	lsd := make([]byte, 7)
	if err := reader.readFull(lsd); err != nil {
		return gifMetadata{}, gifMalformed(err)
	}
	width := int(binary.LittleEndian.Uint16(lsd[0:2]))
	height := int(binary.LittleEndian.Uint16(lsd[2:4]))
	if width == 0 || height == 0 {
		return gifMetadata{}, gifMalformed(fmt.Errorf("zero logical dimensions"))
	}
	if width > gifMaxDimension || height > gifMaxDimension {
		return gifMetadata{}, gifPreflightError(gifErrorDimensionsExceeded, "GIF dimensions exceed 4096 pixels", nil)
	}
	if lsd[4]&0x80 != 0 {
		if err := reader.discard(gifColorTableBytes(lsd[4])); err != nil {
			return gifMetadata{}, gifMalformed(err)
		}
	}

	meta := gifMetadata{Width: width, Height: height}
	var pendingDelay time.Duration
	for {
		marker, err := reader.readByte()
		if err != nil {
			return gifMetadata{}, gifMalformed(err)
		}
		switch marker {
		case 0x3B: // trailer
			if meta.Frames == 0 {
				return gifMetadata{}, gifMalformed(fmt.Errorf("GIF has no image frames"))
			}
			return meta, nil
		case 0x21: // extension
			label, err := reader.readByte()
			if err != nil {
				return gifMetadata{}, gifMalformed(err)
			}
			if label != 0xF9 {
				if err := reader.skipSubBlocks(); err != nil {
					return gifMetadata{}, gifMalformed(err)
				}
				continue
			}
			blockSize, err := reader.readByte()
			if err != nil || blockSize != 4 {
				return gifMetadata{}, gifMalformed(fmt.Errorf("invalid graphics control extension"))
			}
			gce := make([]byte, 4)
			if err := reader.readFull(gce); err != nil {
				return gifMetadata{}, gifMalformed(err)
			}
			terminator, err := reader.readByte()
			if err != nil || terminator != 0 {
				return gifMetadata{}, gifMalformed(fmt.Errorf("invalid graphics control terminator"))
			}
			pendingDelay = time.Duration(binary.LittleEndian.Uint16(gce[1:3])) * 10 * time.Millisecond
		case 0x2C: // image descriptor
			descriptor := make([]byte, 9)
			if err := reader.readFull(descriptor); err != nil {
				return gifMetadata{}, gifMalformed(err)
			}
			left := int(binary.LittleEndian.Uint16(descriptor[0:2]))
			top := int(binary.LittleEndian.Uint16(descriptor[2:4]))
			frameWidth := int(binary.LittleEndian.Uint16(descriptor[4:6]))
			frameHeight := int(binary.LittleEndian.Uint16(descriptor[6:8]))
			if frameWidth == 0 || frameHeight == 0 || left+frameWidth > width || top+frameHeight > height {
				return gifMetadata{}, gifMalformed(fmt.Errorf("invalid image descriptor bounds"))
			}
			if descriptor[8]&0x80 != 0 {
				if err := reader.discard(gifColorTableBytes(descriptor[8])); err != nil {
					return gifMetadata{}, gifMalformed(err)
				}
			}
			if _, err := reader.readByte(); err != nil { // LZW minimum code size
				return gifMetadata{}, gifMalformed(err)
			}
			if err := reader.skipSubBlocks(); err != nil {
				return gifMetadata{}, gifMalformed(err)
			}
			meta.Frames++
			if meta.Frames > gifMaxFrames {
				return gifMetadata{}, gifPreflightError(gifErrorFrameCountExceeded, "GIF contains more than 2000 frames", nil)
			}
			meta.DecodedPixels = int64(width) * int64(height) * int64(meta.Frames)
			if meta.DecodedPixels > gifMaxDecodedPixels {
				return gifMetadata{}, gifPreflightError(gifErrorDecodeBudgetExceeded, "GIF decoded pixel budget exceeds 1.5 billion", nil)
			}
			meta.CycleDuration += pendingDelay
			pendingDelay = 0
			if meta.CycleDuration > gifMaxCycleDuration {
				return gifMetadata{}, gifPreflightError(gifErrorDurationExceeded, "GIF animation cycle exceeds 60 seconds", nil)
			}
		default:
			return gifMetadata{}, gifMalformed(fmt.Errorf("unknown GIF block marker 0x%02x", marker))
		}
	}
}

func gifColorTableBytes(packed byte) int64 {
	return int64(3 * (1 << ((packed & 0x07) + 1)))
}

func gifPreflightError(code, message string, cause error) error {
	return &gifProcessingError{Code: code, Message: message, Cause: cause}
}

func gifMalformed(cause error) error {
	return gifPreflightError(gifErrorProbeFailed, "GIF metadata could not be safely read", cause)
}

type gifMetadataReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *gifMetadataReader) check() error { return r.ctx.Err() }

func (r *gifMetadataReader) readFull(target []byte) error {
	if err := r.check(); err != nil {
		return err
	}
	_, err := io.ReadFull(r.reader, target)
	return err
}

func (r *gifMetadataReader) readByte() (byte, error) {
	var one [1]byte
	err := r.readFull(one[:])
	return one[0], err
}

func (r *gifMetadataReader) discard(size int64) error {
	if err := r.check(); err != nil {
		return err
	}
	if size < 0 {
		return fmt.Errorf("negative discard size")
	}
	var scratch [512]byte
	for size > 0 {
		if err := r.check(); err != nil {
			return err
		}
		chunk := int64(len(scratch))
		if size < chunk {
			chunk = size
		}
		if _, err := io.ReadFull(r.reader, scratch[:chunk]); err != nil {
			return err
		}
		size -= chunk
	}
	return nil
}

func (r *gifMetadataReader) skipSubBlocks() error {
	for {
		size, err := r.readByte()
		if err != nil {
			return err
		}
		if size == 0 {
			return nil
		}
		if err := r.discard(int64(size)); err != nil {
			return err
		}
	}
}
