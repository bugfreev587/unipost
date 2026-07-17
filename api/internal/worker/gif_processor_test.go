package worker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBuildGIFRenderPlanPreservesCompleteCycles(t *testing.T) {
	tests := []struct {
		name     string
		cycle    time.Duration
		loops    int
		duration time.Duration
	}{
		{name: "static", cycle: 0, loops: 1, duration: 5 * time.Second},
		{name: "short", cycle: 4900 * time.Millisecond, loops: 2, duration: 9800 * time.Millisecond},
		{name: "exact five", cycle: 5 * time.Second, loops: 1, duration: 5 * time.Second},
		{name: "long", cycle: 17 * time.Second, loops: 1, duration: 17 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := buildGIFRenderPlan(gifMetadata{Width: 320, Height: 240, Frames: 1, CycleDuration: tt.cycle})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Loops != tt.loops || plan.Duration != tt.duration {
				t.Fatalf("plan = %#v", plan)
			}
		})
	}
}

func TestFFmpegGIFProcessorCreatesValidatedUniversalMP4(t *testing.T) {
	ffmpegPath, ffprobePath := requireGIFBinaries(t)
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "transparent.gif")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	writeAnimatedGIF(t, inputPath)

	processor := &ffmpegGIFProcessor{ffmpegPath: ffmpegPath, ffprobePath: ffprobePath}
	result, err := processor.Process(context.Background(), gifProcessRequest{
		InputPath:       inputPath,
		OutputPath:      outputPath,
		BackgroundColor: "#12ABEF",
	})
	if err != nil {
		if probe, probeErr := processor.probeOutput(context.Background(), outputPath); probeErr == nil {
			t.Logf("rejected probe = %#v", probe)
		}
		t.Fatal(err)
	}
	if result.Width != 4 || result.Height != 2 || result.DurationMS < 5000 || result.DurationMS > 5100 || result.SizeBytes <= 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestValidateGIFOutputProbeRejectsWrongProfile(t *testing.T) {
	for name, probe := range map[string]gifOutputProbe{
		"container": {FormatName: "webm", VideoCodec: "h264", PixelFormat: "yuv420p", Width: 2, Height: 2, FPS: 30, DurationMS: 5000},
		"codec":     {FormatName: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "hevc", PixelFormat: "yuv420p", Width: 2, Height: 2, FPS: 30, DurationMS: 5000},
		"pixel":     {FormatName: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "h264", PixelFormat: "yuv444p", Width: 2, Height: 2, FPS: 30, DurationMS: 5000},
		"fps":       {FormatName: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "h264", PixelFormat: "yuv420p", Width: 2, Height: 2, FPS: 29.97, DurationMS: 5000},
		"audio":     {FormatName: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "h264", PixelFormat: "yuv420p", Width: 2, Height: 2, FPS: 30, DurationMS: 5000, HasAudio: true},
		"odd":       {FormatName: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "h264", PixelFormat: "yuv420p", Width: 3, Height: 2, FPS: 30, DurationMS: 5000},
	} {
		t.Run(name, func(t *testing.T) {
			var processingErr *gifProcessingError
			if err := validateGIFOutputProbe(probe, gifRenderPlan{Width: 2, Height: 2, Duration: 5 * time.Second}, 100); !errors.As(err, &processingErr) || processingErr.Code != gifErrorOutputInvalid {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateGIFOutputProbeUsesStableOutputSizeError(t *testing.T) {
	probe := gifOutputProbe{
		FormatName: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "h264", PixelFormat: "yuv420p",
		Width: 2, Height: 2, FPS: 30, DurationMS: 5000,
	}
	var processingErr *gifProcessingError
	err := validateGIFOutputProbe(
		probe,
		gifRenderPlan{Width: 2, Height: 2, Duration: 5 * time.Second},
		gifOutputHardCapBytes+1,
	)
	if !errors.As(err, &processingErr) || processingErr.Code != gifErrorOutputSizeExceeded {
		t.Fatalf("error = %#v, want %q", err, gifErrorOutputSizeExceeded)
	}
}

func TestClassifyGIFFFmpegRunFailureDistinguishesDecodeFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.mp4")
	if err := classifyGIFFFmpegRunFailure(missing, errors.New("ffmpeg exited")); err.Code != gifErrorDecodeFailed {
		t.Fatalf("missing output code = %q, want %q", err.Code, gifErrorDecodeFailed)
	}
	partial := filepath.Join(t.TempDir(), "partial.mp4")
	if writeErr := os.WriteFile(partial, []byte("partial"), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if err := classifyGIFFFmpegRunFailure(partial, errors.New("ffmpeg exited")); err.Code != gifErrorProcessingFailed {
		t.Fatalf("partial output code = %q, want %q", err.Code, gifErrorProcessingFailed)
	}
}

func requireGIFBinaries(t *testing.T) (string, string) {
	t.Helper()
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	return ffmpegPath, ffprobePath
}

func writeAnimatedGIF(t *testing.T, path string) {
	t.Helper()
	palette := color.Palette{color.RGBA{0, 0, 0, 0}, color.RGBA{255, 0, 0, 255}}
	first := image.NewPaletted(image.Rect(0, 0, 4, 3), palette)
	second := image.NewPaletted(image.Rect(0, 0, 4, 3), palette)
	first.SetColorIndex(0, 0, 1)
	second.SetColorIndex(3, 2, 1)
	var body bytes.Buffer
	if err := gif.EncodeAll(&body, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{25, 25}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBuildGIFRenderPlanScalesWithoutUpscalingAndMakesDimensionsEven(t *testing.T) {
	for _, tt := range []struct {
		width, height int
		wantW, wantH  int
	}{
		{width: 320, height: 241, wantW: 320, wantH: 240},
		{width: 1919, height: 1079, wantW: 1918, wantH: 1078},
		{width: 3840, height: 2160, wantW: 1920, wantH: 1080},
		{width: 2160, height: 3840, wantW: 1080, wantH: 1920},
	} {
		plan, err := buildGIFRenderPlan(gifMetadata{Width: tt.width, Height: tt.height, Frames: 1})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Width != tt.wantW || plan.Height != tt.wantH {
			t.Fatalf("%dx%d => %dx%d, want %dx%d", tt.width, tt.height, plan.Width, plan.Height, tt.wantW, tt.wantH)
		}
	}
}

func TestBuildGIFFFmpegArgsUsesUniversalMP4Profile(t *testing.T) {
	plan := gifRenderPlan{Width: 320, Height: 240, Loops: 2, Duration: 9800 * time.Millisecond}
	args, err := buildGIFFFmpegArgs("/private/input", "/private/output", "#12ABEF", gifMetadata{Width: 321, Height: 241}, plan)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-ignore_loop 1", "-stream_loop -1", "color=c=0x12ABEF:s=321x241:r=30", "overlay=shortest=0:eof_action=repeat",
		"fps=30", "scale=320:240", "-c:v libx264", "-pix_fmt yuv420p", "-an", "-movflags +faststart",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %v", want, args)
		}
	}
	if !slices.Contains(args, "9.800") {
		t.Fatalf("args missing complete-cycle duration: %v", args)
	}
}
