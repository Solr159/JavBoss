package autosub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"javboss/internal/models"
)

type fakeGateway struct {
	offline     bool
	result      DownloadResult
	downloadErr error
	requests    []DownloadRequest
}

func (f *fakeGateway) Ping(context.Context) error {
	if f.offline {
		return errors.New("gateway offline")
	}
	return nil
}

func (f *fakeGateway) Download(_ context.Context, req DownloadRequest) (DownloadResult, error) {
	f.requests = append(f.requests, req)
	if f.downloadErr != nil {
		return DownloadResult{}, f.downloadErr
	}
	return f.result, nil
}

type fakeLocations struct {
	locations []models.VideoLocation
}

func (f *fakeLocations) ListAutoSubtitleLocations(_ context.Context, afterID int64, limit int) ([]models.VideoLocation, error) {
	var out []models.VideoLocation
	for _, loc := range f.locations {
		if loc.ID <= afterID {
			continue
		}
		if len(out) >= limit {
			break
		}
		out = append(out, loc)
	}
	return out, nil
}

func (f *fakeLocations) ListRecentAutoSubtitleLocations(_ context.Context, limit int) ([]models.VideoLocation, error) {
	out := append([]models.VideoLocation(nil), f.locations...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type fakeConfig struct {
	values map[string]string
}

func (f fakeConfig) ListConfig(context.Context) (map[string]string, error) {
	return f.values, nil
}

func TestHasSubtitleSidecar(t *testing.T) {
	tests := []struct {
		name    string
		sidecar string
		want    bool
	}{
		{name: "exact srt", sidecar: "ABC-123.srt", want: true},
		{name: "language suffix ass", sidecar: "ABC-123.zh.ass", want: true},
		{name: "uppercase extension", sidecar: "ABC-123.SRT", want: true},
		{name: "similar code", sidecar: "ABC-1234.srt", want: false},
		{name: "unrelated file", sidecar: "cover.jpg", want: false},
		{name: "subtitle for another video", sidecar: "ABC-124.srt", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			videoPath := filepath.Join(dir, "ABC-123.mp4")
			if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
				t.Fatalf("write video: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, tc.sidecar), []byte("subtitle"), 0o644); err != nil {
				t.Fatalf("write sidecar: %v", err)
			}
			got, err := hasSubtitleSidecar(videoPath)
			if err != nil {
				t.Fatalf("hasSubtitleSidecar() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("hasSubtitleSidecar() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAutoSweepDownloadsOnlyVideosWithoutSubtitles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "ABC-123.mp4"), "video")
	writeTestFile(t, filepath.Join(dir, "ABC-124.mp4"), "video")
	writeTestFile(t, filepath.Join(dir, "ABC-124.zh.srt"), "subtitle")

	gateway := &fakeGateway{result: DownloadResult{Success: true, SubtitlePath: filepath.Join(dir, "ABC-123.zh.srt")}}
	service := NewAutoService(gateway, &fakeLocations{locations: []models.VideoLocation{
		{ID: 1, Filename: "ABC-123.mp4", RelativePath: "ABC-123.mp4", DirectoryRef: models.Directory{Path: dir}},
		{ID: 2, Filename: "ABC-124.mp4", RelativePath: "ABC-124.mp4", DirectoryRef: models.Directory{Path: dir}},
	}}, fakeConfig{values: map[string]string{
		AutoEnabledKey:  "true",
		AutoLanguageKey: "English",
	}})

	result, err := service.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep() error = %v", err)
	}
	if result.Scanned != 2 || result.Downloaded != 1 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(gateway.requests) != 1 {
		t.Fatalf("download requests = %d, want 1", len(gateway.requests))
	}
	request := gateway.requests[0]
	if request.Video.Path != filepath.Join(dir, "ABC-123.mp4") {
		t.Fatalf("downloaded %q, want %q", request.Video.Path, filepath.Join(dir, "ABC-123.mp4"))
	}
	if request.Language != "English" {
		t.Fatalf("language = %q, want English", request.Language)
	}
	if request.Video.SizeBytes != int64(len("video")) {
		t.Fatalf("size = %d, want %d", request.Video.SizeBytes, len("video"))
	}
}

func TestAutoSweepProcessesNewVideosBeforeBacklogCompletes(t *testing.T) {
	dir := t.TempDir()
	var locations []models.VideoLocation
	for i := 1; i <= 11; i++ {
		name := fmt.Sprintf("old-%02d.mp4", i)
		writeTestFile(t, filepath.Join(dir, name), "video")
		writeTestFile(t, filepath.Join(dir, fmt.Sprintf("old-%02d.srt", i)), "subtitle")
		locations = append(locations, models.VideoLocation{
			ID: int64(i), Filename: name, RelativePath: name, DirectoryRef: models.Directory{Path: dir},
		})
	}
	writeTestFile(t, filepath.Join(dir, "new.mp4"), "video")
	locations = append(locations, models.VideoLocation{
		ID: 100, Filename: "new.mp4", RelativePath: "new.mp4", DirectoryRef: models.Directory{Path: dir},
	})

	gateway := &fakeGateway{result: DownloadResult{Success: true}}
	service := NewAutoService(gateway, &fakeLocations{locations: locations}, fakeConfig{values: map[string]string{
		AutoEnabledKey: "true",
	}})
	// Simulate a sweep that is still working through the backlog.
	service.cursor = 1

	result, err := service.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep() error = %v", err)
	}
	if result.Downloaded != 1 || len(gateway.requests) != 1 {
		t.Fatalf("result = %+v, downloads = %d", result, len(gateway.requests))
	}
	if gateway.requests[0].Video.Path != filepath.Join(dir, "new.mp4") {
		t.Fatalf("downloaded %q, want new.mp4", gateway.requests[0].Video.Path)
	}
}

func TestAutoSweepSkipsRecentFailures(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "ABC-125.mp4"), "video")

	gateway := &fakeGateway{result: DownloadResult{Success: false, Error: "no provider"}}
	service := NewAutoService(gateway, &fakeLocations{locations: []models.VideoLocation{
		{ID: 1, Filename: "ABC-125.mp4", RelativePath: "ABC-125.mp4", DirectoryRef: models.Directory{Path: dir}},
	}}, fakeConfig{values: map[string]string{AutoEnabledKey: "true"}})

	first, err := service.Sweep(context.Background())
	if err != nil {
		t.Fatalf("first Sweep() error = %v", err)
	}
	if first.Failed != 1 {
		t.Fatalf("first sweep failed = %d, want 1", first.Failed)
	}

	second, err := service.Sweep(context.Background())
	if err != nil {
		t.Fatalf("second Sweep() error = %v", err)
	}
	if second.Skipped != 1 || second.Failed != 0 {
		t.Fatalf("second sweep = %+v, want 1 skipped and 0 failed", second)
	}
	if len(gateway.requests) != 1 {
		t.Fatalf("download requests = %d, want 1", len(gateway.requests))
	}
}

func TestAutoSweepDisabledDoesNotContactGateway(t *testing.T) {
	gateway := &fakeGateway{offline: true}
	service := NewAutoService(gateway, &fakeLocations{}, fakeConfig{values: map[string]string{
		AutoEnabledKey: "false",
	}})

	result, err := service.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep() error = %v", err)
	}
	if result.Enabled {
		t.Fatalf("expected disabled sweep, got %+v", result)
	}
	if len(gateway.requests) != 0 {
		t.Fatalf("download requests = %d, want 0", len(gateway.requests))
	}
	if status := service.Status(); status.Running || status.LastRun != nil {
		t.Fatalf("disabled sweeps should not publish a run summary: %+v", status)
	}
}

func TestAutoSweepReportsUnreachableGateway(t *testing.T) {
	gateway := &fakeGateway{offline: true}
	service := NewAutoService(gateway, &fakeLocations{}, fakeConfig{values: map[string]string{
		AutoEnabledKey: "true",
	}})

	if _, err := service.Sweep(context.Background()); err == nil {
		t.Fatal("Sweep() error = nil, want gateway error")
	}
	if len(gateway.requests) != 0 {
		t.Fatalf("download requests = %d, want 0", len(gateway.requests))
	}
}

func TestAutoSweepSkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	gateway := &fakeGateway{result: DownloadResult{Success: true}}
	service := NewAutoService(gateway, &fakeLocations{locations: []models.VideoLocation{
		{ID: 1, Filename: "gone.mp4", RelativePath: "gone.mp4", DirectoryRef: models.Directory{Path: dir}},
	}}, fakeConfig{values: map[string]string{AutoEnabledKey: "true"}})

	result, err := service.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep() error = %v", err)
	}
	if result.Skipped != 1 || len(gateway.requests) != 0 {
		t.Fatalf("result = %+v, downloads = %d", result, len(gateway.requests))
	}
}

func TestRunNowSweepsWhenAutomaticallyDisabled(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "ABC-126.mp4"), "video")

	gateway := &fakeGateway{result: DownloadResult{Success: true}}
	service := NewAutoService(gateway, &fakeLocations{locations: []models.VideoLocation{
		{ID: 1, Filename: "ABC-126.mp4", RelativePath: "ABC-126.mp4", DirectoryRef: models.Directory{Path: dir}},
	}}, fakeConfig{values: map[string]string{AutoEnabledKey: "false"}})

	result, err := service.RunNow(context.Background())
	if err != nil {
		t.Fatalf("RunNow() error = %v", err)
	}
	if result.Downloaded != 1 || len(gateway.requests) != 1 {
		t.Fatalf("result = %+v, downloads = %d", result, len(gateway.requests))
	}

	status := service.Status()
	if status.Running {
		t.Fatal("status should report an idle sweep after RunNow returns")
	}
	if status.LastRun == nil {
		t.Fatal("expected a last run summary")
	}
	if status.LastRun.Scanned != 1 || status.LastRun.Downloaded != 1 || status.LastRun.Failed != 0 {
		t.Fatalf("unexpected last run: %+v", status.LastRun)
	}
	if status.LastRun.Enabled {
		t.Fatal("last run should mirror the stored setting")
	}
	if status.LastRun.StartedAt.IsZero() || status.LastRun.FinishedAt.Before(status.LastRun.StartedAt) {
		t.Fatalf("unexpected run timestamps: %+v", status.LastRun)
	}
}

type blockingGateway struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingGateway) Ping(context.Context) error { return nil }

func (b *blockingGateway) Download(ctx context.Context, _ DownloadRequest) (DownloadResult, error) {
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	select {
	case <-b.release:
		return DownloadResult{Success: true}, nil
	case <-ctx.Done():
		return DownloadResult{}, ctx.Err()
	}
}

func TestRunAsyncRejectsConcurrentSweeps(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "ABC-127.mp4"), "video")

	gateway := &blockingGateway{started: make(chan struct{}), release: make(chan struct{})}
	service := NewAutoService(gateway, &fakeLocations{locations: []models.VideoLocation{
		{ID: 1, Filename: "ABC-127.mp4", RelativePath: "ABC-127.mp4", DirectoryRef: models.Directory{Path: dir}},
	}}, fakeConfig{values: map[string]string{AutoEnabledKey: "true"}})

	if !service.RunAsync(context.Background()) {
		t.Fatal("first RunAsync() = false, want true")
	}
	<-gateway.started

	if service.RunAsync(context.Background()) {
		t.Fatal("second RunAsync() = true, want false while a sweep is in flight")
	}
	if _, err := service.Sweep(context.Background()); !errors.Is(err, ErrSweepRunning) {
		t.Fatalf("Sweep() error = %v, want ErrSweepRunning", err)
	}
	if status := service.Status(); !status.Running || status.LastRun != nil {
		t.Fatalf("unexpected status during run: %+v", status)
	}

	close(gateway.release)
	deadline := time.Now().Add(5 * time.Second)
	for service.Status().Running {
		if time.Now().After(deadline) {
			t.Fatal("background sweep did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if last := service.Status().LastRun; last == nil || last.Downloaded != 1 {
		t.Fatalf("unexpected last run: %+v", last)
	}
}

func TestSweepStatusRecordsFailures(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "ABC-128.mp4"), "video")

	gateway := &fakeGateway{offline: true}
	service := NewAutoService(gateway, &fakeLocations{locations: []models.VideoLocation{
		{ID: 1, Filename: "ABC-128.mp4", RelativePath: "ABC-128.mp4", DirectoryRef: models.Directory{Path: dir}},
	}}, fakeConfig{values: map[string]string{AutoEnabledKey: "true"}})

	if _, err := service.Sweep(context.Background()); err == nil {
		t.Fatal("Sweep() error = nil, want gateway error")
	}
	last := service.Status().LastRun
	if last == nil || last.Error == "" {
		t.Fatalf("expected the failure in the last run summary, got %+v", last)
	}
	if last.ErrorCode != SweepErrorGatewayUnavailable {
		t.Fatalf("error code = %q, want %q", last.ErrorCode, SweepErrorGatewayUnavailable)
	}
	if service.Status().Running {
		t.Fatal("a failed sweep must not stay marked as running")
	}
}

func TestFinishRunRecordsErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{name: "gateway", err: newSweepError(SweepErrorGatewayUnavailable, errors.New("boom")), wantCode: SweepErrorGatewayUnavailable},
		{name: "settings", err: newSweepError(SweepErrorSettingsUnavailable, errors.New("boom")), wantCode: SweepErrorSettingsUnavailable},
		{name: "locations", err: newSweepError(SweepErrorLocationsUnavailable, errors.New("boom")), wantCode: SweepErrorLocationsUnavailable},
		{name: "uncoded", err: errors.New("boom"), wantCode: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := NewAutoService(&fakeGateway{}, &fakeLocations{}, fakeConfig{})
			service.finishRun(time.Now(), AutoSweepResult{}, tc.err)
			last := service.Status().LastRun
			if last == nil {
				t.Fatal("finishRun did not publish a summary")
			}
			if last.ErrorCode != tc.wantCode {
				t.Fatalf("error code = %q, want %q", last.ErrorCode, tc.wantCode)
			}
			if last.Error != tc.err.Error() {
				t.Fatalf("error = %q, want %q", last.Error, tc.err.Error())
			}
		})
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
