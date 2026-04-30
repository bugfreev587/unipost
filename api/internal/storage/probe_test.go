package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	mp4 "github.com/abema/go-mp4"
)

// TestProbeVideoNilClient locks down the documented "nil *Client is a
// valid value" contract for the new ProbeVideo method, matching the
// other methods covered in TestNilClient (media_test.go).
func TestProbeVideoNilClient(t *testing.T) {
	var c *Client
	if _, err := c.ProbeVideo(context.TODO(), "k"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("ProbeVideo on nil: want ErrNotConfigured, got %v", err)
	}
}

// TestParseVideoMetadata exercises the IO-free half of ProbeVideo
// against a hand-built mp4 with a known mvhd duration + tkhd visual
// dimensions. Building the fixture in code (instead of checking in a
// binary mp4) keeps the test readable and lets us tweak the values
// without re-encoding anything.
func TestParseVideoMetadata(t *testing.T) {
	const (
		// 30s at timescale 1000 → 30000 movie-time units.
		wantTimescale uint32 = 1000
		wantDuration  uint64 = 30000
		wantDurMS            = 30000

		// 1080×1920 vertical 9:16 — the aspect ratio Facebook
		// reclassifies into Reels, which is exactly the case PR2's
		// validator needs to catch.
		wantWidth  uint16 = 1080
		wantHeight uint16 = 1920
	)

	mp4Bytes := buildMinimalMP4(t, wantTimescale, wantDuration, wantWidth, wantHeight)

	meta := parseVideoMetadata(bytes.NewReader(mp4Bytes))
	if meta.Width != int(wantWidth) {
		t.Errorf("Width: got %d, want %d", meta.Width, wantWidth)
	}
	if meta.Height != int(wantHeight) {
		t.Errorf("Height: got %d, want %d", meta.Height, wantHeight)
	}
	if meta.DurationMS != wantDurMS {
		t.Errorf("DurationMS: got %d, want %d", meta.DurationMS, wantDurMS)
	}
}

// TestParseVideoMetadataNonMP4 documents the "no metadata, no error"
// contract for inputs that aren't a parseable mp4 — random bytes,
// truncated files, webm. The hydrate path relies on this so it can
// still flip a non-mp4 video's row to 'uploaded' instead of stalling
// it in 'pending' forever.
func TestParseVideoMetadataNonMP4(t *testing.T) {
	cases := map[string][]byte{
		"empty":      {},
		"junk":       []byte("not an mp4 file at all, just text"),
		"truncated":  {0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p'}, // claims 24 bytes, only has 8
		"big_sizes":  bytes.Repeat([]byte{0x7F}, 64),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			meta := parseVideoMetadata(bytes.NewReader(data))
			if meta.Width != 0 || meta.Height != 0 || meta.DurationMS != 0 {
				t.Errorf("expected zero metadata on garbage input %q, got %+v", name, meta)
			}
		})
	}
}

// TestParseVideoMetadataAudioOnlyTrack guards against a regression
// where an audio-only mp4 (no visual track) returns 0×0. tkhd width
// and height are zero for audio tracks, so the loop's `w*h > 0`
// check should leave Width/Height at zero rather than picking the
// audio track. The validator uses HasDimensions() to decide whether
// to apply placement rules; an audio file misclassified as 0×0
// "valid" would silently pass FB's aspect-ratio check.
func TestParseVideoMetadataAudioOnlyTrack(t *testing.T) {
	mp4Bytes := buildMinimalMP4(t, 1000, 5000, 0, 0)
	meta := parseVideoMetadata(bytes.NewReader(mp4Bytes))
	if meta.Width != 0 || meta.Height != 0 {
		t.Errorf("audio-only track: want 0×0, got %dx%d", meta.Width, meta.Height)
	}
	// Duration should still come through (it's mvhd-level, not tkhd).
	if meta.DurationMS != 5000 {
		t.Errorf("DurationMS: got %d, want 5000", meta.DurationMS)
	}
}

// buildMinimalMP4 hand-rolls the smallest mp4 that exercises both
// boxes parseVideoMetadata cares about: moov/mvhd (for duration) and
// moov/trak/tkhd (for dimensions). No mdat — there's no actual video
// payload, but the parser doesn't need one to read the metadata.
func buildMinimalMP4(t *testing.T, timescale uint32, duration uint64, width, height uint16) []byte {
	t.Helper()

	buf := &seekableBuffer{}
	w := mp4.NewWriter(buf)

	// ftyp: minimum brand declaration so the file looks like a
	// real ISOBMFF container. mp42 is a generic brand any parser
	// (including go-mp4) will accept.
	if _, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeFtyp()}); err != nil {
		t.Fatalf("start ftyp: %v", err)
	}
	if _, err := mp4.Marshal(w, &mp4.Ftyp{
		MajorBrand:   [4]byte{'m', 'p', '4', '2'},
		MinorVersion: 0,
		CompatibleBrands: []mp4.CompatibleBrandElem{
			{CompatibleBrand: [4]byte{'i', 's', 'o', 'm'}},
		},
	}, mp4.Context{}); err != nil {
		t.Fatalf("marshal ftyp: %v", err)
	}
	if _, err := w.EndBox(); err != nil {
		t.Fatalf("end ftyp: %v", err)
	}

	// moov: container for mvhd + trak.
	if _, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMoov()}); err != nil {
		t.Fatalf("start moov: %v", err)
	}

	// mvhd: top-level duration. Use version 0 (32-bit fields).
	if _, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMvhd()}); err != nil {
		t.Fatalf("start mvhd: %v", err)
	}
	if _, err := mp4.Marshal(w, &mp4.Mvhd{
		Timescale:  timescale,
		DurationV0: uint32(duration),
		Rate:       0x00010000,
		Volume:     0x0100,
	}, mp4.Context{}); err != nil {
		t.Fatalf("marshal mvhd: %v", err)
	}
	if _, err := w.EndBox(); err != nil {
		t.Fatalf("end mvhd: %v", err)
	}

	// trak/tkhd: per-track header with visual dimensions.
	if _, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeTrak()}); err != nil {
		t.Fatalf("start trak: %v", err)
	}
	if _, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeTkhd()}); err != nil {
		t.Fatalf("start tkhd: %v", err)
	}
	if _, err := mp4.Marshal(w, &mp4.Tkhd{
		TrackID:    1,
		DurationV0: uint32(duration),
		// Width and Height are 16.16 fixed-point in the mp4 spec —
		// shift the integer pixel value up to occupy the high half
		// of the uint32 so go-mp4's GetWidthInt() reads it back.
		Width:  uint32(width) << 16,
		Height: uint32(height) << 16,
	}, mp4.Context{}); err != nil {
		t.Fatalf("marshal tkhd: %v", err)
	}
	if _, err := w.EndBox(); err != nil {
		t.Fatalf("end tkhd: %v", err)
	}
	if _, err := w.EndBox(); err != nil {
		t.Fatalf("end trak: %v", err)
	}

	if _, err := w.EndBox(); err != nil {
		t.Fatalf("end moov: %v", err)
	}

	return buf.Bytes()
}

// seekableBuffer is a tiny in-memory io.WriteSeeker so tests don't
// need a real file. mp4.NewWriter requires Seek (to back-patch box
// sizes after EndBox), which *bytes.Buffer doesn't provide.
type seekableBuffer struct {
	buf []byte
	pos int64
}

func (s *seekableBuffer) Write(p []byte) (int, error) {
	end := s.pos + int64(len(p))
	if end > int64(len(s.buf)) {
		// Grow the underlying slice, zero-filling any gap between
		// the old end and the new write position (rare — mp4.Writer
		// only seeks backward to back-patch sizes).
		grown := make([]byte, end)
		copy(grown, s.buf)
		s.buf = grown
	}
	copy(s.buf[s.pos:], p)
	s.pos = end
	return len(p), nil
}

func (s *seekableBuffer) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = s.pos + offset
	case io.SeekEnd:
		abs = int64(len(s.buf)) + offset
	default:
		return 0, errors.New("seekableBuffer: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("seekableBuffer: negative position")
	}
	s.pos = abs
	return abs, nil
}

func (s *seekableBuffer) Bytes() []byte {
	return s.buf
}
