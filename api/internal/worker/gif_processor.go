package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	gifMinimumOutputDuration = 5 * time.Second
	gifMaximumOutputEdge     = 1920
	gifProcessingTimeout     = 5 * time.Minute
	gifOutputHardCapBytes    = int64(4 * 1024 * 1024 * 1024)
)

var gifBackgroundPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type gifRenderPlan struct {
	Width    int
	Height   int
	Loops    int
	Duration time.Duration
}

type gifProcessRequest struct {
	InputPath       string
	OutputPath      string
	BackgroundColor string
}

type gifProcessResult struct {
	SizeBytes  int64
	Width      int
	Height     int
	DurationMS int
}

type gifOutputProbe struct {
	FormatName  string
	VideoCodec  string
	PixelFormat string
	Width       int
	Height      int
	FPS         float64
	DurationMS  int
	HasAudio    bool
}

type gifProcessor interface {
	Process(context.Context, gifProcessRequest) (gifProcessResult, error)
}

type ffmpegGIFProcessor struct {
	ffmpegPath  string
	ffprobePath string
}

func newFFmpegGIFProcessor() *ffmpegGIFProcessor {
	ffmpegPath := strings.TrimSpace(os.Getenv("FFMPEG_PATH"))
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	ffprobePath := strings.TrimSpace(os.Getenv("FFPROBE_PATH"))
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	return &ffmpegGIFProcessor{ffmpegPath: ffmpegPath, ffprobePath: ffprobePath}
}

func (p *ffmpegGIFProcessor) Process(parent context.Context, req gifProcessRequest) (gifProcessResult, error) {
	ctx, cancel := context.WithTimeout(parent, gifProcessingTimeout)
	defer cancel()

	input, err := os.Open(req.InputPath)
	if err != nil {
		return gifProcessResult{}, gifPreflightError(gifErrorProbeFailed, "GIF input could not be opened", err)
	}
	stat, err := input.Stat()
	if err != nil {
		_ = input.Close()
		return gifProcessResult{}, gifPreflightError(gifErrorProbeFailed, "GIF input metadata could not be read", err)
	}
	meta, err := preflightGIF(ctx, input, stat.Size())
	closeErr := input.Close()
	if err != nil {
		return gifProcessResult{}, err
	}
	if closeErr != nil {
		return gifProcessResult{}, gifPreflightError(gifErrorProbeFailed, "GIF input could not be closed", closeErr)
	}
	plan, err := buildGIFRenderPlan(meta)
	if err != nil {
		return gifProcessResult{}, err
	}
	args, err := buildGIFFFmpegArgs(req.InputPath, req.OutputPath, req.BackgroundColor, meta, plan)
	if err != nil {
		return gifProcessResult{}, gifPreflightError(gifErrorProcessingFailed, "GIF render plan is invalid", err)
	}
	if _, runErr := exec.CommandContext(ctx, p.ffmpegPath, args...).CombinedOutput(); runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return gifProcessResult{}, &gifProcessingError{Code: gifErrorProcessingTimeout, Message: "GIF conversion exceeded the five minute processing limit", Retryable: true, Cause: ctx.Err()}
		}
		return gifProcessResult{}, &gifProcessingError{Code: gifErrorProcessingFailed, Message: "GIF conversion failed", Cause: runErr}
	}
	probe, err := p.probeOutput(ctx, req.OutputPath)
	if err != nil {
		return gifProcessResult{}, err
	}
	outputStat, err := os.Stat(req.OutputPath)
	if err != nil {
		return gifProcessResult{}, &gifProcessingError{Code: gifErrorOutputInvalid, Message: "converted MP4 could not be read", Cause: err}
	}
	if err := validateGIFOutputProbe(probe, plan, outputStat.Size()); err != nil {
		return gifProcessResult{}, err
	}
	return gifProcessResult{SizeBytes: outputStat.Size(), Width: probe.Width, Height: probe.Height, DurationMS: probe.DurationMS}, nil
}

func (p *ffmpegGIFProcessor) probeOutput(ctx context.Context, path string) (gifOutputProbe, error) {
	cmd := exec.CommandContext(ctx, p.ffprobePath,
		"-v", "error",
		"-show_entries", "format=format_name,duration:stream=codec_type,codec_name,pix_fmt,width,height,avg_frame_rate",
		"-of", "json",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return gifOutputProbe{}, &gifProcessingError{Code: gifErrorOutputInvalid, Message: "converted MP4 could not be probed", Cause: err}
	}
	var payload struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			PixelFormat string `json:"pix_fmt"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			FrameRate   string `json:"avg_frame_rate"`
		} `json:"streams"`
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return gifOutputProbe{}, &gifProcessingError{Code: gifErrorOutputInvalid, Message: "converted MP4 probe response was invalid", Cause: err}
	}
	probe := gifOutputProbe{FormatName: payload.Format.Name}
	for _, stream := range payload.Streams {
		switch stream.CodecType {
		case "audio":
			probe.HasAudio = true
		case "video":
			if probe.VideoCodec != "" {
				return gifOutputProbe{}, gifPreflightError(gifErrorOutputInvalid, "converted MP4 contains more than one video stream", nil)
			}
			probe.VideoCodec = stream.CodecName
			probe.PixelFormat = stream.PixelFormat
			probe.Width = stream.Width
			probe.Height = stream.Height
			probe.FPS = parseGIFFrameRate(stream.FrameRate)
		}
	}
	if seconds, parseErr := strconv.ParseFloat(payload.Format.Duration, 64); parseErr == nil && seconds > 0 {
		probe.DurationMS = int(math.Round(seconds * 1000))
	}
	return probe, nil
}

func parseGIFFrameRate(value string) float64 {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		fps, _ := strconv.ParseFloat(value, 64)
		return fps
	}
	numerator, err1 := strconv.ParseFloat(parts[0], 64)
	denominator, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func validateGIFOutputProbe(probe gifOutputProbe, plan gifRenderPlan, sizeBytes int64) error {
	validContainer := false
	for _, name := range strings.Split(probe.FormatName, ",") {
		if name == "mp4" {
			validContainer = true
			break
		}
	}
	valid := validContainer && probe.VideoCodec == "h264" && probe.PixelFormat == "yuv420p" &&
		probe.Width == plan.Width && probe.Height == plan.Height && probe.Width%2 == 0 && probe.Height%2 == 0 &&
		math.Abs(probe.FPS-30) < 0.01 && !probe.HasAudio && probe.DurationMS > 0 &&
		math.Abs(float64(probe.DurationMS)-float64(plan.Duration.Milliseconds())) <= 100 &&
		sizeBytes > 0 && sizeBytes <= gifOutputHardCapBytes
	if !valid {
		return &gifProcessingError{Code: gifErrorOutputInvalid, Message: "converted MP4 does not match the universal_mp4_v1 profile"}
	}
	return nil
}

func buildGIFRenderPlan(meta gifMetadata) (gifRenderPlan, error) {
	if meta.Width <= 0 || meta.Height <= 0 || meta.Frames <= 0 {
		return gifRenderPlan{}, fmt.Errorf("GIF render metadata is incomplete")
	}
	scale := math.Min(1, float64(gifMaximumOutputEdge)/float64(max(meta.Width, meta.Height)))
	width := evenFloor(int(math.Floor(float64(meta.Width) * scale)))
	height := evenFloor(int(math.Floor(float64(meta.Height) * scale)))
	if width < 2 || height < 2 {
		return gifRenderPlan{}, gifPreflightError(gifErrorDimensionsExceeded, "GIF dimensions are too small for H.264 output", nil)
	}

	loops := 1
	duration := meta.CycleDuration
	if duration <= 0 {
		duration = gifMinimumOutputDuration
	} else if duration < gifMinimumOutputDuration {
		loops = int(math.Ceil(float64(gifMinimumOutputDuration) / float64(duration)))
		duration *= time.Duration(loops)
	}
	return gifRenderPlan{Width: width, Height: height, Loops: loops, Duration: duration}, nil
}

func evenFloor(value int) int { return value - value%2 }

func buildGIFFFmpegArgs(inputPath, outputPath, backgroundColor string, meta gifMetadata, plan gifRenderPlan) ([]string, error) {
	if strings.TrimSpace(inputPath) == "" || strings.TrimSpace(outputPath) == "" {
		return nil, fmt.Errorf("GIF FFmpeg input and output paths are required")
	}
	if !gifBackgroundPattern.MatchString(backgroundColor) {
		return nil, fmt.Errorf("invalid GIF background color")
	}
	if meta.Width <= 0 || meta.Height <= 0 || plan.Width <= 0 || plan.Height <= 0 || plan.Loops <= 0 || plan.Duration <= 0 {
		return nil, fmt.Errorf("invalid GIF render plan")
	}

	durationSeconds := fmt.Sprintf("%.3f", plan.Duration.Seconds())
	background := strings.TrimPrefix(strings.ToUpper(backgroundColor), "#")
	filter := fmt.Sprintf(
		"[1:v][0:v]overlay=shortest=0:eof_action=repeat:format=auto,fps=30,scale=%d:%d:flags=lanczos,setsar=1,format=yuv420p[v]",
		plan.Width,
		plan.Height,
	)
	return []string{
		"-y",
		"-ignore_loop", "1",
		// Keep the demuxer feeding complete animation cycles. The exact complete-
		// cycle duration is enforced by -t below; a finite stream_loop can lose
		// the final GIF frame when FFmpeg regenerates loop timestamps.
		"-stream_loop", "-1",
		"-i", inputPath,
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=0x%s:s=%dx%d:r=30:d=%s", background, meta.Width, meta.Height, durationSeconds),
		"-filter_complex", filter,
		"-map", "[v]",
		"-t", durationSeconds,
		"-an",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-r", "30",
		"-movflags", "+faststart",
		outputPath,
	}, nil
}
