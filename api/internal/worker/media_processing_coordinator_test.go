package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestMediaProcessingCoordinatorAlternatesKindsWithOneClaimPerRun(t *testing.T) {
	queries := &fakeMediaCoordinatorQueries{jobs: map[string][]db.MediaProcessingJob{
		mediaAudioOverlayKind:  {{ID: "audio_1", Kind: mediaAudioOverlayKind}, {ID: "audio_2", Kind: mediaAudioOverlayKind}},
		mediaGIFConversionKind: {{ID: "gif_1", Kind: mediaGIFConversionKind}, {ID: "gif_2", Kind: mediaGIFConversionKind}},
	}}
	audio := &fakeClaimedMediaProcessor{}
	gif := &fakeClaimedMediaProcessor{}
	coordinator := NewMediaProcessingCoordinator(queries, audio, gif)

	for range 4 {
		coordinator.RunOnce(context.Background())
	}
	if got := queries.claimedKinds; len(got) != 4 || got[0] != mediaAudioOverlayKind || got[1] != mediaGIFConversionKind || got[2] != mediaAudioOverlayKind || got[3] != mediaGIFConversionKind {
		t.Fatalf("claim order = %v", got)
	}
	if len(audio.jobs) != 2 || len(gif.jobs) != 2 {
		t.Fatalf("processed audio=%v gif=%v", audio.jobs, gif.jobs)
	}
}

func TestMediaProcessingCoordinatorFallsBackWithoutStarvingPreferredKind(t *testing.T) {
	queries := &fakeMediaCoordinatorQueries{jobs: map[string][]db.MediaProcessingJob{
		mediaGIFConversionKind: {{ID: "gif_1", Kind: mediaGIFConversionKind}},
	}}
	audio := &fakeClaimedMediaProcessor{}
	gif := &fakeClaimedMediaProcessor{}
	coordinator := NewMediaProcessingCoordinator(queries, audio, gif)
	coordinator.RunOnce(context.Background())
	if len(gif.jobs) != 1 || len(queries.claimedKinds) != 2 || queries.claimedKinds[0] != mediaAudioOverlayKind || queries.claimedKinds[1] != mediaGIFConversionKind {
		t.Fatalf("claims=%v processed=%v", queries.claimedKinds, gif.jobs)
	}
}

func TestMediaProcessingCoordinatorSerializesConcurrentRunOnce(t *testing.T) {
	queries := &fakeMediaCoordinatorQueries{jobs: map[string][]db.MediaProcessingJob{
		mediaAudioOverlayKind: {{ID: "audio_1", Kind: mediaAudioOverlayKind}, {ID: "audio_2", Kind: mediaAudioOverlayKind}},
	}}
	processor := &fakeClaimedMediaProcessor{entered: make(chan struct{}), release: make(chan struct{})}
	coordinator := NewMediaProcessingCoordinator(queries, processor, &fakeClaimedMediaProcessor{})
	var wait sync.WaitGroup
	wait.Add(1)
	go func() { defer wait.Done(); coordinator.RunOnce(context.Background()) }()
	<-processor.entered
	coordinator.RunOnce(context.Background())
	close(processor.release)
	wait.Wait()
	if len(processor.jobs) != 1 {
		t.Fatalf("concurrent RunOnce processed %d jobs", len(processor.jobs))
	}
}

type fakeMediaCoordinatorQueries struct {
	mu            sync.Mutex
	jobs          map[string][]db.MediaProcessingJob
	claimedKinds  []string
	promotedKinds []string
	recoveryCalls int
}

func (f *fakeMediaCoordinatorQueries) PromoteDueMediaProcessingRetriesByKind(_ context.Context, kind string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.promotedKinds = append(f.promotedKinds, kind)
	return 0, nil
}
func (f *fakeMediaCoordinatorQueries) RecoverStaleMediaProcessingJobs(context.Context, int32) ([]db.MediaProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recoveryCalls++
	return nil, nil
}
func (f *fakeMediaCoordinatorQueries) TouchMediaProcessingJobHeartbeat(context.Context, string) (int64, error) {
	return 1, nil
}
func (f *fakeMediaCoordinatorQueries) ClaimMediaProcessingJobsByKind(_ context.Context, arg db.ClaimMediaProcessingJobsByKindParams) ([]db.MediaProcessingJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimedKinds = append(f.claimedKinds, arg.JobKind)
	queue := f.jobs[arg.JobKind]
	if len(queue) == 0 {
		return nil, nil
	}
	job := queue[0]
	f.jobs[arg.JobKind] = queue[1:]
	return []db.MediaProcessingJob{job}, nil
}

type fakeClaimedMediaProcessor struct {
	mu      sync.Mutex
	jobs    []string
	entered chan struct{}
	release chan struct{}
}

func (f *fakeClaimedMediaProcessor) ProcessClaimedJob(_ context.Context, job db.MediaProcessingJob) error {
	f.mu.Lock()
	f.jobs = append(f.jobs, job.ID)
	first := len(f.jobs) == 1
	f.mu.Unlock()
	if first && f.entered != nil {
		close(f.entered)
		<-f.release
	}
	return nil
}
