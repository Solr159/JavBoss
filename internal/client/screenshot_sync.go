package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
)

const (
	screenshotSyncInterval   = 300 * time.Millisecond
	screenshotSyncMaxBackoff = time.Minute
	screenshotSyncIdleTime   = time.Minute
	screenshotResumeInterval = 30 * time.Second
)

type screenshotSyncJob struct {
	client  *Client
	ctx     context.Context
	videoID int64
	dir     string
	wake    chan struct{}

	mu                 sync.RWMutex
	cookie             string
	credentialsChanged bool
	lastActivated      time.Time
	files              map[string]*screenshotFileState
}

type screenshotFileState struct {
	size        int64
	modifiedNS  int64
	stableScans int
	failures    int
	nextAttempt time.Time
	uploaded    bool
}

func remoteStorageKey(remoteURL string) string {
	digest := sha256.Sum256([]byte(strings.TrimRight(strings.TrimSpace(remoteURL), "/")))
	return hex.EncodeToString(digest[:8])
}

func (c *Client) startScreenshotSync(videoID int64, cookie, dataDir string) error {
	if videoID <= 0 || strings.TrimSpace(cookie) == "" {
		return nil
	}
	dir := filepath.Join(dataDir, "video", strconv.FormatInt(videoID, 10), "screenshot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create client screenshot sync directory: %w", err)
	}

	c.screenshotMu.Lock()
	defer c.screenshotMu.Unlock()
	if c.screenshotClosed {
		return nil
	}
	c.screenshotCookie = cookie
	if job := c.screenshotJobs[videoID]; job != nil && filepath.Clean(job.dir) == filepath.Clean(dir) {
		job.activate(cookie)
		return nil
	}
	job := &screenshotSyncJob{
		client:        c,
		ctx:           c.screenshotCtx,
		videoID:       videoID,
		dir:           dir,
		wake:          make(chan struct{}, 1),
		cookie:        cookie,
		lastActivated: time.Now(),
		files:         make(map[string]*screenshotFileState),
	}
	c.screenshotJobs[videoID] = job
	c.screenshotWG.Add(1)
	go func() {
		defer c.screenshotWG.Done()
		job.run()
	}()
	return nil
}

func (c *Client) startScreenshotResumeLoop() {
	c.screenshotWG.Add(1)
	go func() {
		defer c.screenshotWG.Done()
		ticker := time.NewTicker(screenshotResumeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.screenshotCtx.Done():
				return
			case <-ticker.C:
				c.screenshotMu.Lock()
				cookie := c.screenshotCookie
				closed := c.screenshotClosed
				c.screenshotMu.Unlock()
				if !closed && cookie != "" {
					c.resumePendingScreenshotSync(cookie)
				}
			}
		}
	}()
}

func (c *Client) closeScreenshotSync() {
	c.screenshotMu.Lock()
	if !c.screenshotClosed {
		c.screenshotClosed = true
		c.screenshotCancel()
	}
	c.screenshotMu.Unlock()
	c.screenshotWG.Wait()
}

func (c *Client) refreshScreenshotCookies(cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	c.screenshotMu.Lock()
	c.screenshotCookie = mergeCookieHeader(c.screenshotCookie, cookies)
	jobs := make([]*screenshotSyncJob, 0, len(c.screenshotJobs))
	for _, job := range c.screenshotJobs {
		jobs = append(jobs, job)
	}
	shouldResume := time.Since(c.screenshotLastResume) >= 30*time.Second
	if shouldResume {
		c.screenshotLastResume = time.Now()
	}
	c.screenshotMu.Unlock()
	for _, job := range jobs {
		job.mu.RLock()
		current := job.cookie
		job.mu.RUnlock()
		job.updateCookie(mergeCookieHeader(current, cookies))
	}
	if freshCookie := c.latestScreenshotCookie(); shouldResume && freshCookie != "" {
		c.resumePendingScreenshotSync(freshCookie)
	}
}

func (c *Client) latestScreenshotCookie() string {
	c.screenshotMu.Lock()
	defer c.screenshotMu.Unlock()
	return c.screenshotCookie
}

func (c *Client) resumePendingScreenshotSync(cookie string) {
	videoRoot := filepath.Join(c.clientDataDir(), "video")
	entries, err := os.ReadDir(videoRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Error("read pending client screenshot root failed: %v", err)
		}
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		videoID, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || videoID <= 0 {
			continue
		}
		screenshotDir := filepath.Join(videoRoot, entry.Name(), "screenshot")
		if !directoryHasPendingScreenshots(screenshotDir) {
			continue
		}
		if err := c.startScreenshotSync(videoID, cookie, c.clientDataDir()); err != nil {
			logging.Error("resume pending client screenshot sync failed video_id=%d: %v", videoID, err)
		}
	}
}

func directoryHasPendingScreenshots(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && isClientScreenshotName(entry.Name()) {
			return true
		}
	}
	return false
}

func (j *screenshotSyncJob) updateCookie(cookie string) {
	j.mu.Lock()
	if j.cookie == cookie {
		j.mu.Unlock()
		return
	}
	j.cookie = cookie
	j.credentialsChanged = true
	j.mu.Unlock()
	select {
	case j.wake <- struct{}{}:
	default:
	}
}

func (j *screenshotSyncJob) activate(cookie string) {
	j.mu.Lock()
	if j.cookie != cookie {
		j.cookie = cookie
		j.credentialsChanged = true
	}
	j.lastActivated = time.Now()
	j.mu.Unlock()
	select {
	case j.wake <- struct{}{}:
	default:
	}
}

func (j *screenshotSyncJob) lastActivatedAt() time.Time {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.lastActivated
}

func (j *screenshotSyncJob) run() {
	ticker := time.NewTicker(screenshotSyncInterval)
	defer ticker.Stop()
	for {
		pending := j.syncOnce()
		if !pending && j.client.retireScreenshotJob(j, time.Now()) {
			return
		}
		select {
		case <-j.ctx.Done():
			return
		case <-j.wake:
		case <-ticker.C:
		}
	}
}

func (c *Client) retireScreenshotJob(job *screenshotSyncJob, now time.Time) bool {
	c.screenshotMu.Lock()
	defer c.screenshotMu.Unlock()
	if c.screenshotJobs[job.videoID] != job || now.Sub(job.lastActivatedAt()) < screenshotSyncIdleTime {
		return false
	}
	delete(c.screenshotJobs, job.videoID)
	return true
}

func (j *screenshotSyncJob) syncOnce() bool {
	if j.takeCredentialsChanged() {
		for _, state := range j.files {
			if !state.uploaded {
				state.nextAttempt = time.Time{}
				state.failures = 0
			}
		}
	}
	entries, err := os.ReadDir(j.dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Error("read client screenshot sync directory failed: %v", err)
			return true
		}
		return false
	}
	now := time.Now()
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isClientScreenshotName(name) {
			continue
		}
		seen[name] = struct{}{}
		info, err := entry.Info()
		if err != nil || info.Size() <= 0 {
			continue
		}
		modifiedNS := info.ModTime().UnixNano()
		state := j.files[name]
		if state == nil || state.size != info.Size() || state.modifiedNS != modifiedNS {
			j.files[name] = &screenshotFileState{size: info.Size(), modifiedNS: modifiedNS}
			continue
		}
		path := filepath.Join(j.dir, name)
		if state.uploaded {
			if err := removeUploadedScreenshotIfUnchanged(path, state); err != nil {
				logging.Error("remove uploaded client screenshot failed: %v", err)
			}
			continue
		}
		state.stableScans++
		if state.stableScans < 1 || now.Before(state.nextAttempt) {
			continue
		}
		if err := j.upload(path, name, state); err != nil {
			state.failures++
			state.nextAttempt = now.Add(screenshotRetryBackoff(state.failures))
			logging.Error("upload client screenshot failed video_id=%d name=%s: %v", j.videoID, name, err)
			continue
		}
		state.uploaded = true
		if err := removeUploadedScreenshotIfUnchanged(path, state); err != nil {
			logging.Error("remove uploaded client screenshot failed: %v", err)
		}
	}
	for name := range j.files {
		if _, ok := seen[name]; !ok {
			delete(j.files, name)
		}
	}
	return len(seen) > 0 || len(j.files) > 0
}

func (j *screenshotSyncJob) takeCredentialsChanged() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	changed := j.credentialsChanged
	j.credentialsChanged = false
	return changed
}

func (j *screenshotSyncJob) upload(path, name string, state *screenshotFileState) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	j.mu.RLock()
	cookie := j.cookie
	j.mu.RUnlock()
	if cookie == "" {
		return fmt.Errorf("remote authentication is unavailable")
	}
	headers := make(http.Header)
	headers.Set("Cookie", cookie)
	headers.Set("Content-Type", screenshotContentType(name))
	headers.Set("Content-Length", strconv.FormatInt(state.size, 10))
	requestPath := "/videos/" + strconv.FormatInt(j.videoID, 10) + "/screenshots/" + url.PathEscape(name)
	response, err := j.client.remoteRequest(j.ctx, http.MethodPut, requestPath, "", file, headers)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("remote screenshot upload returned HTTP %d", response.StatusCode)
	}
	return nil
}

func removeUploadedScreenshotIfUnchanged(path string, state *screenshotFileState) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() != state.size || info.ModTime().UnixNano() != state.modifiedNS {
		state.uploaded = false
		state.stableScans = 0
		state.size = info.Size()
		state.modifiedNS = info.ModTime().UnixNano()
		return nil
	}
	return os.Remove(path)
}

func screenshotRetryBackoff(failures int) time.Duration {
	if failures < 1 {
		return time.Second
	}
	shift := failures - 1
	if shift > 6 {
		shift = 6
	}
	delay := time.Second * time.Duration(1<<shift)
	if delay > screenshotSyncMaxBackoff {
		return screenshotSyncMaxBackoff
	}
	return delay
}

func isClientScreenshotName(name string) bool {
	if strings.TrimSpace(name) == "" || filepath.Base(name) != name || !strings.HasPrefix(name, "mpv_") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func screenshotContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func mergeCookieHeader(current string, updates []*http.Cookie) string {
	values := make(map[string]string)
	header := make(http.Header)
	if strings.TrimSpace(current) != "" {
		header.Set("Cookie", current)
	}
	request := &http.Request{Header: header}
	for _, cookie := range request.Cookies() {
		values[cookie.Name] = cookie.Value
	}
	for _, cookie := range updates {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		if cookie.MaxAge < 0 {
			delete(values, cookie.Name)
			continue
		}
		values[cookie.Name] = cookie.Value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, (&http.Cookie{Name: name, Value: values[name]}).String())
	}
	return strings.Join(parts, "; ")
}
