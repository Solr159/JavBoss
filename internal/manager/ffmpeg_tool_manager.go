package manager

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/runtimeconfig"
	"javboss/internal/util"
)

const ffmpegRelease = "6.1.1"

type ffmpegDownload struct {
	url    string
	sha256 string
}

var ffmpegDownloads = map[string]ffmpegDownload{
	"windows/amd64": {
		url:    "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-win32-x64.gz",
		sha256: "8883a3dffbd0a16cf4ef95206ea05283f78908dbfb118f73c83f4951dcc06d77",
	},
	"linux/amd64": {
		url:    "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-linux-x64.gz",
		sha256: "bfe8a8fc511530457b528c48d77b5737527b504a3797a9bc4866aeca69c2dffa",
	},
	"darwin/amd64": {
		url:    "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-x64.gz",
		sha256: "929b375c1182d956c51f7ac25e0b2b0411fb01f6f407aa15c9758efeb4242106",
	},
	"darwin/arm64": {
		url:    "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-arm64.gz",
		sha256: "8923876afa8db5585022d7860ec7e589af192f441c56793971276d450ed3bbfa",
	},
}

// FFmpegToolStatus describes the project-local FFmpeg installation.
type FFmpegToolStatus struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Supported       bool   `json:"supported"`
	Installed       bool   `json:"installed"`
	Source          string `json:"source,omitempty"`
	Downloading     bool   `json:"downloading"`
	Progress        int    `json:"progress"`
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	Path            string `json:"path"`
	Error           string `json:"error,omitempty"`
}

// FFmpegToolManager downloads FFmpeg into the running project's persistent data/tools directory.
type FFmpegToolManager struct {
	mu sync.Mutex

	context     context.Context
	targetPath  string
	displayPath string
	bundledDir  string
	tempDir     string
	downloadURL string
	downloadSHA string
	httpClient  *http.Client

	containerMode bool
	resolveFFmpeg func() (string, error)

	downloading     bool
	progress        int
	downloadedBytes int64
	totalBytes      int64
	downloadError   string
}

// NewFFmpegToolManager creates a manager for the current platform.
func NewFFmpegToolManager(ctx context.Context, baseDir string) *FFmpegToolManager {
	if ctx == nil {
		ctx = context.Background()
	}
	relativePath := util.FFmpegToolRelativePath()
	download := ffmpegDownloads[runtime.GOOS+"/"+runtime.GOARCH]
	return &FFmpegToolManager{
		context:       ctx,
		targetPath:    filepath.Join(baseDir, relativePath),
		displayPath:   filepath.ToSlash(relativePath),
		bundledDir:    filepath.Join(baseDir, "internal", "bin"),
		tempDir:       os.TempDir(),
		downloadURL:   download.url,
		downloadSHA:   download.sha256,
		httpClient:    util.NewHTTPClient(0),
		containerMode: runtimeconfig.ContainerMode(),
		resolveFFmpeg: util.ResolveFFmpegPath,
	}
}

// Status returns a snapshot of the current installation and download state.
func (m *FFmpegToolManager) Status() FFmpegToolStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, source := m.detectInstallation()
	version := ""
	if source == "downloaded" {
		version = ffmpegRelease
	}
	return FFmpegToolStatus{
		Name:            "ffmpeg",
		Version:         version,
		Supported:       m.downloadURL != "",
		Installed:       installed,
		Source:          source,
		Downloading:     m.downloading,
		Progress:        m.progress,
		DownloadedBytes: m.downloadedBytes,
		TotalBytes:      m.totalBytes,
		Path:            m.displayPath,
		Error:           m.downloadError,
	}
}

// StartDownload starts an asynchronous download. It returns false when FFmpeg
// is already installed or a download is already running.
func (m *FFmpegToolManager) StartDownload() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, _ := m.detectInstallation()
	if m.downloading || installed {
		return false, nil
	}
	if m.downloadURL == "" {
		return false, errors.New("automatic FFmpeg download is not supported on this platform")
	}

	m.downloading = true
	m.progress = 0
	m.downloadedBytes = 0
	m.totalBytes = 0
	m.downloadError = ""
	go m.download()
	return true, nil
}

func (m *FFmpegToolManager) detectInstallation() (bool, string) {
	if m.resolveFFmpeg != nil {
		if resolvedPath, err := m.resolveFFmpeg(); err == nil && isUsableFFmpegFile(resolvedPath) {
			switch {
			case m.containerMode:
				return true, "builtin"
			case sameFilePath(resolvedPath, m.targetPath):
				return true, "downloaded"
			case pathWithinDirectory(resolvedPath, m.bundledDir):
				return true, "builtin"
			default:
				return true, "system"
			}
		}
	}
	if isUsableFFmpegFile(m.targetPath) {
		return true, "downloaded"
	}
	return false, ""
}

func (m *FFmpegToolManager) download() {
	err := m.downloadToTarget()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloading = false
	if err != nil {
		m.downloadError = err.Error()
		logging.Error("download FFmpeg error: %v", err)
		return
	}
	m.progress = 100
	m.downloadError = ""
	logging.Info("FFmpeg installed at %s", m.targetPath)
}

func (m *FFmpegToolManager) downloadToTarget() error {
	req, err := http.NewRequestWithContext(m.context, http.MethodGet, m.downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", "JavBoss/"+ffmpegRelease)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download FFmpeg: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download FFmpeg: unexpected HTTP status %s", resp.Status)
	}

	m.mu.Lock()
	m.totalBytes = resp.ContentLength
	m.mu.Unlock()

	tempPattern := ".ffmpeg-download-*"
	if strings.EqualFold(filepath.Ext(m.targetPath), ".exe") {
		tempPattern += ".exe"
	}
	tempFile, err := os.CreateTemp(m.tempDir, tempPattern)
	if err != nil {
		return fmt.Errorf("create FFmpeg temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	hasher := sha256.New()
	countedBody := &downloadProgressReader{
		reader: resp.Body,
		update: m.updateProgress,
		hash:   hasher,
	}
	gzipReader, err := gzip.NewReader(countedBody)
	if err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("open FFmpeg archive: %w", err)
	}

	_, copyErr := io.Copy(tempFile, gzipReader)
	gzipErr := gzipReader.Close()
	closeErr := tempFile.Close()
	if copyErr != nil {
		return fmt.Errorf("extract FFmpeg: %w", copyErr)
	}
	if gzipErr != nil {
		return fmt.Errorf("close FFmpeg archive: %w", gzipErr)
	}
	if closeErr != nil {
		return fmt.Errorf("save FFmpeg: %w", closeErr)
	}
	if actualSHA := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(actualSHA, m.downloadSHA) {
		return fmt.Errorf("verify FFmpeg archive checksum: got %s", actualSHA)
	}
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return fmt.Errorf("make FFmpeg executable: %w", err)
	}
	if err := validateFFmpeg(tempPath); err != nil {
		return err
	}

	targetDir := filepath.Dir(m.targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create FFmpeg directory: %w", err)
	}
	if isUsableFFmpegFile(m.targetPath) {
		return nil
	}
	if err := installFFmpegFile(tempPath, m.targetPath); err != nil {
		return fmt.Errorf("install FFmpeg: %w", err)
	}
	return nil
}

func installFFmpegFile(sourcePath string, targetPath string) error {
	return installFFmpegFileWithRename(sourcePath, targetPath, os.Rename)
}

func installFFmpegFileWithRename(
	sourcePath string,
	targetPath string,
	renameFile func(string, string) error,
) error {
	renameErr := renameFile(sourcePath, targetPath)
	if renameErr == nil {
		return nil
	}
	if isUsableFFmpegFile(targetPath) {
		return nil
	}

	stagedPath, err := copyFFmpegToTargetDirectory(sourcePath, targetPath)
	if err != nil {
		return fmt.Errorf("copy FFmpeg to target directory after rename failed (%v): %w", renameErr, err)
	}
	defer os.Remove(stagedPath)

	if err := renameFile(stagedPath, targetPath); err == nil {
		return nil
	} else if isUsableFFmpegFile(targetPath) {
		return nil
	}

	targetInfo, statErr := os.Stat(targetPath)
	switch {
	case statErr == nil && !targetInfo.Mode().IsRegular():
		return fmt.Errorf("replace existing FFmpeg target: target is not a regular file")
	case statErr != nil && !os.IsNotExist(statErr):
		return fmt.Errorf("inspect existing FFmpeg target: %w", statErr)
	}
	if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("remove unusable FFmpeg target: %w", removeErr)
	}
	if err := renameFile(stagedPath, targetPath); err != nil {
		return fmt.Errorf("move staged FFmpeg into place: %w", err)
	}
	return nil
}

func copyFFmpegToTargetDirectory(sourcePath string, targetPath string) (string, error) {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open verified FFmpeg: %w", err)
	}
	defer sourceFile.Close()

	tempPattern := ".ffmpeg-install-*"
	if strings.EqualFold(filepath.Ext(targetPath), ".exe") {
		tempPattern += ".exe"
	}
	stagedFile, err := os.CreateTemp(filepath.Dir(targetPath), tempPattern)
	if err != nil {
		return "", fmt.Errorf("create FFmpeg install file: %w", err)
	}
	stagedPath := stagedFile.Name()
	keepStagedFile := false
	defer func() {
		_ = stagedFile.Close()
		if !keepStagedFile {
			_ = os.Remove(stagedPath)
		}
	}()

	if _, err := io.Copy(stagedFile, sourceFile); err != nil {
		return "", fmt.Errorf("copy verified FFmpeg: %w", err)
	}
	if err := stagedFile.Sync(); err != nil {
		return "", fmt.Errorf("sync FFmpeg install file: %w", err)
	}
	if err := stagedFile.Chmod(0o755); err != nil {
		return "", fmt.Errorf("make FFmpeg install file executable: %w", err)
	}
	if err := stagedFile.Close(); err != nil {
		return "", fmt.Errorf("close FFmpeg install file: %w", err)
	}
	keepStagedFile = true
	return stagedPath, nil
}

func (m *FFmpegToolManager) updateProgress(downloaded int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadedBytes = downloaded
	if m.totalBytes > 0 {
		progress := int(downloaded * 100 / m.totalBytes)
		if progress > 99 {
			progress = 99
		}
		m.progress = progress
	}
}

type downloadProgressReader struct {
	reader     io.Reader
	downloaded int64
	update     func(int64)
	hash       io.Writer
}

func (r *downloadProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		_, _ = r.hash.Write(p[:n])
		r.downloaded += int64(n)
		r.update(r.downloaded)
	}
	return n, err
}

func isUsableFFmpegFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func sameFilePath(left string, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func pathWithinDirectory(path string, directory string) bool {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(directory) == "" {
		return false
	}
	relativePath, err := filepath.Rel(directory, path)
	if err != nil {
		return false
	}
	return relativePath != ".." &&
		relativePath != "." &&
		!strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func validateFFmpeg(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "-version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("validate FFmpeg: %w", err)
	}
	if !strings.Contains(strings.ToLower(string(output)), "ffmpeg version") {
		return errors.New("validate FFmpeg: downloaded file did not report an FFmpeg version")
	}
	return nil
}
