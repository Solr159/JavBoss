package stream

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"javboss/internal/models"
	"javboss/internal/util"
)

func TestPublishOnlyCompleteSegments(t *testing.T) {
	output := t.TempDir()
	staging := t.TempDir()
	s := &runningStream{outputDir: output, streamType: StreamTypeHLS, tp: &transcodeProcess{stagingDir: staging, nextSegment: 3}}
	for path, content := range map[string]string{filepath.Join(output, "3.ts"): "cached", filepath.Join(staging, "3.ts"): "duplicate", filepath.Join(staging, "4.ts.tmp"): "partial"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s.publishSegments()
	b, err := os.ReadFile(filepath.Join(output, "3.ts"))
	if err != nil || string(b) != "cached" || s.tp.nextSegment != 4 || segmentExists(filepath.Join(output, "4.ts")) {
		t.Fatalf("unexpected published files: cached=%q next=%d err=%v", b, s.tp.nextSegment, err)
	}
	if err := os.Rename(filepath.Join(staging, "4.ts.tmp"), filepath.Join(staging, "4.ts")); err != nil {
		t.Fatal(err)
	}
	// The periodic scan must recover even if the filesystem delivered no event.
	sm := newStreamer(output)
	s.lastAccessed = time.Now()
	sm.runningStreams["movie"] = s
	sm.monitorStreams()
	if !segmentExists(filepath.Join(output, "4.ts")) || s.tp.nextSegment != 5 {
		t.Fatal("completed last segment was not published")
	}
}

func TestSegmentRequestsWakeWithoutPolling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a shell executable")
	}
	t.Chdir(t.TempDir())
	t.Setenv("JAVBOSS_BUILD_MODE", "development")
	t.Setenv("JAVBOSS_CONTAINER", "")
	tool := util.FFmpegToolRelativePath()
	if err := os.MkdirAll(filepath.Dir(tool), 0755); err != nil {
		t.Fatal(err)
	}
	// The test drives filesystem publication; this process stays alive until a seek cancels it.
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexec sleep 60\n"), 0755); err != nil {
		t.Fatal(err)
	}
	sm := newStreamer(t.TempDir())
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	sm.watcher = watcher
	go sm.watchSegments(watcher)
	go func() {
		for {
			select {
			case <-sm.context.Done():
				return
			case <-sm.wake:
				sm.monitorStreams()
			}
		}
	}()
	t.Cleanup(sm.Stop)
	request := func(ctx context.Context, idx int) <-chan string {
		done := make(chan string, 1)
		go func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/segment", nil).WithContext(ctx)
			sm.ServeSegment(rec, req, Options{StreamType: StreamTypeHLS, Key: "movie", MediaInfo: &models.MediaInfo{Version: models.MediaInfoVersion, DurationSeconds: 1200}, Segment: strconv.Itoa(idx) + ".ts"})
			done <- rec.Body.String()
		}()
		return done
	}
	waitProcess := func(target, waiters int) string {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			sm.streamsMutex.Lock()
			s := sm.runningStreams["movie_hls"]
			var dir string
			if s != nil && s.tp != nil && s.tp.nextSegment == target && len(s.waitingSegments) == waiters {
				dir = s.tp.stagingDir
			}
			sm.streamsMutex.Unlock()
			if dir != "" {
				return dir
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("no process for segment %d with %d waiters", target, waiters)
		return ""
	}
	expect := func(done <-chan string, want string) {
		t.Helper()
		select {
		case got := <-done:
			if got != want {
				t.Fatalf("segment = %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("request waited for polling")
		}
	}
	publish := func(dir string, idx int, data string) {
		t.Helper()
		final := filepath.Join(dir, strconv.Itoa(idx)+".ts")
		if err := os.WriteFile(final+".tmp", []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(final+".tmp", final); err != nil {
			t.Fatal(err)
		}
	}
	ctx := t.Context()
	a := request(ctx, 1)
	dir := waitProcess(1, 1)
	b := request(ctx, 1)
	if waitProcess(1, 2) != dir {
		t.Fatal("duplicate request restarted FFmpeg")
	}
	publish(dir, 1, "first")
	expect(a, "first")
	expect(b, "first")
	stale, cancel := context.WithCancel(ctx)
	defer cancel()
	skipped := request(stale, 120)
	waitProcess(120, 1)
	cancel()
	expect(skipped, "")
	end := request(ctx, 300)
	dir = waitProcess(300, 1)
	publish(dir, 300, "target")
	expect(end, "target")
	expect(request(ctx, 1), "first")
}

func TestCachedSegmentServedWithoutProbeOrMonitor(t *testing.T) {
	sm := newStreamer(t.TempDir())
	t.Cleanup(sm.Stop)
	dir := filepath.Join(sm.cacheDir, StreamTypeHLS.FileDir("movie", 0))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "3.ts"), []byte("cached"), 0600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/3.ts", nil)
	request.Header.Set("Range", "bytes=1-3")
	rec := httptest.NewRecorder()
	sm.ServeSegment(rec, request, Options{StreamType: StreamTypeHLS, SourcePath: "missing.avi", Duration: 20, Key: "movie", Segment: "3.ts"})
	if rec.Code != 206 || rec.Header().Get("Content-Type") != MimeMpegTS || rec.Body.String() != "ach" {
		t.Fatalf("cached response = %d %q", rec.Code, rec.Body.String())
	}
}
