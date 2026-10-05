package manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"javboss/internal/models"
)

func screenshotTestVideo() *models.Video {
	return &models.Video{ID: 1, DurationSec: 300, ModifiedAt: time.Unix(1710000000, 0), Size: 100}
}

func receiveScreenshot[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for screenshot task")
		var zero T
		return zero
	}
}

func TestScreenshotWaitCancellationDoesNotCancelSharedJob(t *testing.T) {
	m := NewScreenshotManager(t.TempDir(), func(context.Context, int64) (*models.Video, error) { return nil, nil })
	video := screenshotTestVideo()
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { _, err := m.GetThumbnail(ctx, video); result <- err }()
	job := receiveScreenshot(t, m.tasks)
	cancel()
	if err := receiveScreenshot(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait = %v", err)
	}
	// Scan and HTTP requests both reuse the still-running job.
	m.EnqueueThumbnail(video)
	task, _ := screenshotTaskForVideo(video)
	var callers sync.WaitGroup
	for range 20 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			got, err := m.schedule(task, false)
			if got != job || err != nil {
				t.Errorf("duplicate request did not share task: %v", err)
			}
		}()
	}
	callers.Wait()
	if len(m.tasks) != 0 {
		t.Fatal("duplicate tasks were queued")
	}
	path := m.screenshotPath(video.ID, task.Second)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	go func() { _, err := m.GetThumbnail(t.Context(), video); result <- err }()
	if err := os.WriteFile(path, []byte("generated image"), 0600); err != nil {
		t.Fatal(err)
	}
	m.finish(job, nil)
	if err := receiveScreenshot(t, result); err != nil {
		t.Fatalf("remaining caller failed: %v", err)
	}
	if got, err := m.GetThumbnail(t.Context(), video); err != nil || got != path || len(m.tasks) != 0 {
		t.Fatalf("existing screenshot = %q, err=%v", got, err)
	}
}

func TestScreenshotQueueFullAndFailureAllowRetry(t *testing.T) {
	m := NewScreenshotManager(t.TempDir(), func(context.Context, int64) (*models.Video, error) { return nil, nil })
	m.tasks = make(chan *screenshotJob, 1)
	task, _ := screenshotTaskForVideo(screenshotTestVideo())
	first, err := m.schedule(task, false)
	if err != nil {
		t.Fatal(err)
	}
	other := task
	other.VideoID++
	if _, err := m.schedule(other, false); !errors.Is(err, ErrScreenshotQueueFull) {
		t.Fatalf("full queue error = %v", err)
	}
	if duplicate, err := m.schedule(task, false); err != nil || duplicate != first {
		t.Fatal("full queue rejected a shared request")
	}
	<-m.tasks
	failure := errors.New("capture failed")
	m.finish(first, failure)
	<-first.done
	if !errors.Is(first.err, failure) {
		t.Fatalf("task error = %v", first.err)
	}
	if retry, err := m.schedule(task, false); err != nil || retry == first {
		t.Fatalf("failed task prevented retry: %v", err)
	}
}

func TestScreenshotWorkersBoundConcurrencyAndReleaseFailures(t *testing.T) {
	started := make(chan int64, 3)
	release := make(chan struct{})
	failure := errors.New("fetch failed")
	m := NewScreenshotManager(t.TempDir(), func(ctx context.Context, id int64) (*models.Video, error) {
		started <- id
		select {
		case <-release:
			return nil, failure
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	m.workers = 2
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m.Start(ctx)
	m.Start(ctx) // Repeated starts must not add workers.
	var jobs []*screenshotJob
	for id := int64(1); id <= 3; id++ {
		task, _ := screenshotTaskForVideo(screenshotTestVideo())
		task.VideoID = id
		job, err := m.schedule(task, false)
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, job)
	}
	receiveScreenshot(t, started)
	receiveScreenshot(t, started)
	if len(m.tasks) != 1 {
		t.Fatal("more jobs started than the worker limit")
	}
	close(release)
	receiveScreenshot(t, started)
	for _, job := range jobs {
		receiveScreenshot(t, job.done)
		if !errors.Is(job.err, failure) {
			t.Fatalf("worker failure not delivered: %v", job.err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pending) != 0 {
		t.Fatal("failed tasks were not released")
	}
}

func TestScreenshotShutdownReleasesQueuedWaiters(t *testing.T) {
	m := NewScreenshotManager(t.TempDir(), func(ctx context.Context, _ int64) (*models.Video, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	m.workers = 1
	ctx, cancel := context.WithCancel(t.Context())
	m.Start(ctx)
	task, _ := screenshotTaskForVideo(screenshotTestVideo())
	first, _ := m.schedule(task, false)
	task.VideoID++
	second, _ := m.schedule(task, false)
	cancel()
	for _, job := range []*screenshotJob{first, second} {
		receiveScreenshot(t, job.done)
		if job.err == nil {
			t.Fatal("shutdown reported success without a screenshot")
		}
	}
}
