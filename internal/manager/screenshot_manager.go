package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/models"
	"javboss/internal/runtimeconfig"
	"javboss/internal/util"
)

// screenshotTask represents a request to capture a screenshot for a specific video.
type screenshotTask struct {
	VideoID    int64
	Second     int
	ModifiedAt time.Time
	Size       int64
}

// VideoFetcher loads a video record by ID.
type VideoFetcher func(ctx context.Context, id int64) (*models.Video, error)

const maxScreenshotWorkers = 8

var ErrScreenshotUnavailable = errors.New("screenshot manager is unavailable")
var ErrScreenshotQueueFull = errors.New("screenshot queue is full")
var ErrNoThumbnail = errors.New("video has no thumbnail timestamp")
var errScreenshotStale = errors.New("screenshot task metadata is stale")

type screenshotKey struct {
	videoID int64
	second  int
}

type screenshotJob struct {
	task screenshotTask
	done chan struct{}
	err  error
}

// ScreenshotManager coordinates asynchronous screenshot generation using the worker.
type ScreenshotManager struct {
	tasks      chan *screenshotJob
	workers    int
	dataDir    string
	fetchVideo VideoFetcher
	mu         sync.Mutex
	pending    map[screenshotKey]*screenshotJob
	startOnce  sync.Once
	stopped    bool
	stoppedCh  chan struct{}
}

// NewScreenshotManager creates a manager when dataDir and fetchVideo are provided.
// Returns nil when either is missing, effectively disabling screenshot generation.
func NewScreenshotManager(dataDir string, fetchVideo VideoFetcher) *ScreenshotManager {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" || fetchVideo == nil {
		return nil
	}
	workers := runtime.GOMAXPROCS(0)
	if workers <= 0 {
		workers = 1
	}
	if workers > maxScreenshotWorkers {
		workers = maxScreenshotWorkers
	}
	logging.Info("screenshot manager initialized with %d workers", workers)
	return &ScreenshotManager{
		tasks:      make(chan *screenshotJob, 5000),
		workers:    workers,
		dataDir:    dataDir,
		fetchVideo: fetchVideo,
		pending:    make(map[screenshotKey]*screenshotJob),
		stoppedCh:  make(chan struct{}),
	}
}

// Start launches the background worker. Safe to call with nil manager.
func (m *ScreenshotManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	if m.workers <= 0 {
		m.workers = runtime.GOMAXPROCS(0)
		if m.workers <= 0 {
			m.workers = 1
		}
		if m.workers > maxScreenshotWorkers {
			m.workers = maxScreenshotWorkers
		}
	}
	m.startOnce.Do(func() {
		for i := 0; i < m.workers; i++ {
			go m.startWorker(ctx)
		}
		go func() {
			<-ctx.Done()
			m.mu.Lock()
			defer m.mu.Unlock()
			m.stopped = true
			close(m.stoppedCh)
			for key, job := range m.pending {
				job.err = ErrScreenshotUnavailable
				close(job.done)
				delete(m.pending, key)
			}
		}()
	})
}

// schedule deduplicates queued and running tasks. Scans retain their blocking
// queue admission; HTTP requests get a retryable error when the queue is full.
func (m *ScreenshotManager) schedule(task screenshotTask, block bool) (*screenshotJob, error) {
	if m == nil {
		return nil, ErrScreenshotUnavailable
	}
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return nil, ErrScreenshotUnavailable
	}
	key := screenshotKey{task.VideoID, task.Second}
	if job := m.pending[key]; job != nil {
		m.mu.Unlock()
		return job, nil
	}
	job := &screenshotJob{task: task, done: make(chan struct{})}
	m.pending[key] = job
	m.mu.Unlock()
	if block {
		select {
		case m.tasks <- job:
			return job, nil
		case <-m.stoppedCh:
			return nil, ErrScreenshotUnavailable
		}
	}
	select {
	case m.tasks <- job:
		return job, nil
	default:
		m.finish(job, ErrScreenshotQueueFull)
		return nil, ErrScreenshotQueueFull
	}
}

func (m *ScreenshotManager) finish(job *screenshotJob, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := screenshotKey{job.task.VideoID, job.task.Second}
	if m.pending[key] != job {
		return // Shutdown already released the waiters.
	}
	job.err = err
	delete(m.pending, key)
	close(job.done)
}

// GetThumbnail returns an existing default thumbnail or generates it through the
// shared worker queue. Cancelling ctx stops only this caller's wait.
func (m *ScreenshotManager) GetThumbnail(ctx context.Context, video *models.Video) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if m == nil {
		return "", ErrScreenshotUnavailable
	}
	if video == nil || video.ID <= 0 {
		return "", ErrNoThumbnail
	}
	second, ok := pickScreenshotSecond(video.DurationSec)
	if !ok {
		return "", ErrNoThumbnail
	}
	path := m.screenshotPath(video.ID, second)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat screenshot: %w", err)
	}
	task, ok := screenshotTaskForVideo(video)
	if !ok {
		return "", errors.New("video has no valid screenshot task")
	}
	job, err := m.schedule(task, false)
	if err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-job.done:
		if job.err != nil {
			return "", job.err
		}
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("stat generated screenshot: %w", err)
		}
		return path, nil
	}
}

// EnqueueThumbnail schedules default thumbnail generation for a video.
func (m *ScreenshotManager) EnqueueThumbnail(video *models.Video) {
	if m == nil {
		return
	}
	task, ok := screenshotTaskForVideo(video)
	if !ok {
		return
	}
	if _, err := m.schedule(task, true); err != nil {
		logging.Error("enqueue screenshot (video_id=%d): %v", task.VideoID, err)
	}
}

// CaptureFile captures a screenshot for videoPath at second into outputPath.
func (m *ScreenshotManager) CaptureFile(ctx context.Context, videoPath string, second float64, outputPath string) error {
	if m == nil {
		return errors.New("screenshot manager is not configured")
	}
	return m.capture(ctx, videoPath, second, outputPath)
}

// screenshotPath builds the on-disk screenshot path for a video ID and second.
func (m *ScreenshotManager) screenshotPath(videoID int64, second int) string {
	if m == nil {
		return ""
	}
	dataDir := m.dataDir
	if dataDir == "" || videoID <= 0 || second <= 0 {
		return ""
	}
	fileName := fmt.Sprintf("%d.jpg", second)
	return filepath.Join(dataDir, "video", strconv.FormatInt(videoID, 10), "screenshot", fileName)
}

var screenshotSeconds = []int{128, 63, 32, 16, 8, 4, 2, 1}

// pickScreenshotSecond picks the closest configured second that does not exceed durationSec.
func pickScreenshotSecond(durationSec int64) (int, bool) {
	if durationSec <= 0 {
		return 0, false
	}
	for _, candidate := range screenshotSeconds {
		if durationSec >= int64(candidate) {
			return candidate, true
		}
	}
	return 0, false
}

// screenshotTaskForVideo builds a screenshot task for the given video using standard selection logic.
func screenshotTaskForVideo(video *models.Video) (screenshotTask, bool) {
	if video == nil {
		return screenshotTask{}, false
	}
	modifiedAt, size, ok := videoTaskMeta(video)
	if video.ID <= 0 || !ok || modifiedAt.IsZero() {
		return screenshotTask{}, false
	}
	second, ok := pickScreenshotSecond(video.DurationSec)
	if !ok {
		return screenshotTask{}, false
	}
	return screenshotTask{
		VideoID:    video.ID,
		Second:     second,
		ModifiedAt: modifiedAt,
		Size:       size,
	}, true
}

func videoTaskMeta(video *models.Video) (time.Time, int64, bool) {
	if video == nil {
		return time.Time{}, 0, false
	}
	if len(video.Locations) > 0 {
		loc := video.Locations[0]
		return loc.ModifiedAt, video.Size, !loc.ModifiedAt.IsZero()
	}
	return video.ModifiedAt, video.Size, !video.ModifiedAt.IsZero()
}

// startWorker launches a background loop that consumes screenshot generation
// tasks. The worker stops when the context is done or the channel is closed.
func (m *ScreenshotManager) startWorker(ctx context.Context) {
	if m == nil || m.dataDir == "" || m.fetchVideo == nil {
		logging.Info("screenshot worker disabled: data directory or fetcher missing")
		return
	}

	for {
		select {
		case <-ctx.Done():
			logging.Info("screenshot worker exiting: context cancelled")
			return
		case job, ok := <-m.tasks:
			if !ok {
				logging.Info("screenshot worker exiting: task channel closed")
				return
			}
			err := m.processTask(ctx, job.task)
			m.finish(job, err)
			if err != nil {
				logging.Error("screenshot task failed (video_id=%d, second=%d): %v", job.task.VideoID, job.task.Second, err)
			}
		}
	}
}

func (m *ScreenshotManager) processTask(parent context.Context, task screenshotTask) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if task.VideoID <= 0 || task.Second <= 0 || task.ModifiedAt.IsZero() {
		return errors.New("invalid screenshot task: missing video id, second, or modified_at")
	}
	screenshotPath := m.screenshotPath(task.VideoID, task.Second)
	if screenshotPath == "" {
		return errors.New("invalid screenshot task: missing screenshot path")
	}
	if _, err := os.Stat(screenshotPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat screenshot: %w", err)
	}

	video, err := m.fetchVideo(parent, task.VideoID)
	if err != nil {
		return err
	}
	if video == nil {
		return os.ErrNotExist
	}
	modifiedAt, size, ok := videoTaskMeta(video)
	if !ok || !sameVideoMeta(modifiedAt, size, task) {
		return errScreenshotStale
	}

	videoPath, err := resolveVideoPath(video)
	if err != nil {
		return err
	}

	info, err := os.Stat(videoPath)
	if err != nil {
		return err
	}
	if !sameVideoMeta(info.ModTime(), info.Size(), task) {
		return errScreenshotStale
	}

	// Bound mpv execution time to avoid stuck processes.
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()

	return m.capture(ctx, videoPath, float64(task.Second), screenshotPath)
}

func (m *ScreenshotManager) capture(ctx context.Context, videoPath string, second float64, outputPath string) error {
	if videoPath == "" {
		return errors.New("video path is required")
	}
	if second < 0 {
		return errors.New("second is required")
	}
	if outputPath == "" {
		return errors.New("output path is required")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("ensure screenshot dir: %w", err)
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(outputPath), ".screenshot-*")
	if err != nil {
		return fmt.Errorf("create screenshot temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	shotPath := filepath.Join(tempDir, "00000001.jpg")

	if runtime.GOOS == "darwin" || runtimeconfig.ContainerMode() {
		ffmpegPath, err := util.ResolveFFmpegPath()
		if err != nil {
			return fmt.Errorf("resolve ffmpeg path: %w", err)
		}
		if err := runFFmpegScreenshot(ctx, ffmpegPath, videoPath, second, shotPath); err != nil {
			return err
		}
		return moveScreenshot(shotPath, outputPath)
	}

	mpvPath, pathErr := util.ResolveMPVPath()
	if pathErr != nil {
		return fmt.Errorf("resolve mpv path: %w", pathErr)
	}
	args := buildMPVScreenshotArgs(second, tempDir, videoPath)

	// Suppress Windows' startup busy cursor when background screenshots repeatedly
	// launch the GUI mpv.exe; hiding the console alone does not disable that feedback.
	out, err := util.BackgroundCombinedOutput(ctx, mpvPath, args...)
	if err != nil {
		_ = os.Remove(shotPath)
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mpv not found: %w", err)
		}
		lastOut := strings.TrimSpace(string(out))
		if lastOut != "" {
			return fmt.Errorf("mpv screenshot failed: %w: %s", err, lastOut)
		}
		return fmt.Errorf("mpv screenshot failed: %w", err)
	}

	info, err := os.Stat(shotPath)
	if err != nil {
		return errors.New("mpv produced no screenshot file")
	}
	if info.Size() == 0 {
		_ = os.Remove(shotPath)
		return errors.New("mpv produced empty screenshot file")
	}

	return moveScreenshot(shotPath, outputPath)
}

func runFFmpegScreenshot(ctx context.Context, ffmpegPath string, videoPath string, second float64, outputPath string) error {
	args := buildFFmpegScreenshotArgs(second, outputPath, videoPath)
	cmd := util.BackgroundCommandContext(ctx, ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(outputPath)
		lastOut := strings.TrimSpace(string(out))
		if lastOut != "" {
			return fmt.Errorf("ffmpeg screenshot failed: %w: %s", err, lastOut)
		}
		return fmt.Errorf("ffmpeg screenshot failed: %w", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return errors.New("ffmpeg produced no screenshot file")
	}
	if info.Size() == 0 {
		_ = os.Remove(outputPath)
		return errors.New("ffmpeg produced empty screenshot file")
	}
	return nil
}

func moveScreenshot(shotPath string, outputPath string) error {
	if err := os.Rename(shotPath, outputPath); err != nil {
		return fmt.Errorf("rename screenshot: %w", err)
	}
	return nil
}

func buildFFmpegScreenshotArgs(second float64, outputPath string, videoPath string) []string {
	return []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-ss", formatScreenshotSecond(second),
		"-i", videoPath,
		"-map", "0:v:0",
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,
	}
}

func buildMPVScreenshotArgs(second float64, tempDir string, videoPath string) []string {
	return []string{
		"--no-config",
		"--really-quiet",
		"--msg-level=all=error",
		"--ao=null",
		"--hr-seek=yes",
		"--start=" + formatScreenshotSecond(second),
		"--frames=1",
		"--vo=image",
		"--vo-image-format=jpg",
		"--vo-image-outdir=" + tempDir,
		videoPath,
	}
}

func formatScreenshotSecond(second float64) string {
	if second == float64(int64(second)) {
		return strconv.FormatInt(int64(second), 10)
	}
	return strconv.FormatFloat(second, 'f', 3, 64)
}

func resolveVideoPath(video *models.Video) (string, error) {
	if video == nil {
		return "", errors.New("video is nil")
	}
	if len(video.Locations) > 0 {
		loc := video.Locations[0]
		dirPath := strings.TrimSpace(loc.DirectoryRef.Path)
		relPath := strings.TrimSpace(loc.RelativePath)
		if dirPath != "" && relPath != "" {
			return filepath.Join(dirPath, filepath.FromSlash(relPath)), nil
		}
	}
	return "", errors.New("video location missing")
}

func sameVideoMeta(modifiedAt time.Time, size int64, task screenshotTask) bool {
	if task.ModifiedAt.IsZero() {
		return false
	}
	if size != task.Size {
		return false
	}
	return modifiedAt.Equal(task.ModifiedAt)
}
