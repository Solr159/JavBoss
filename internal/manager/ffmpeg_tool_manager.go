package manager

import (
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

const ffmpegRelease = "8.1.2-50-g1a748fe2cd-20260831"

type ffmpegDownload struct {
	version      string
	url          string
	downloadSHA  string
	binarySHA256 string
}

// Keep Windows/Linux sources and both checksums aligned with scripts/cli/cli.mjs.
// BtbN month-end builds are retained for two years; Linux needs glibc >= 2.28
// and kernel >= 4.18. These builds include NVENC, QSV, AMF and Linux VAAPI.
var ffmpegDownloads = map[string]ffmpegDownload{
	"windows/amd64": {
		version:      ffmpegRelease,
		url:          "https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-08-31-13-27/ffmpeg-n8.1.2-50-g1a748fe2cd-win64-gpl-8.1.zip",
		downloadSHA:  "273abb45f3f9f76c303e35ff39f5bb6c23c163ae65f6244a32b7d4a7f6cf0616",
		binarySHA256: "19121c4a9dece4780f33e6cfc2ba58e36347d4c64f0df4efc05a6959a8191aa6",
	},
	"linux/amd64": {
		version:      ffmpegRelease,
		url:          "https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-08-31-13-27/ffmpeg-n8.1.2-50-g1a748fe2cd-linux64-gpl-8.1.tar.xz",
		downloadSHA:  "c733b4b2951e5957e15505f788b2c65a7a41b6da4b289e295852cc38079b4d2b",
		binarySHA256: "ad7a8c8e8fe4f50972f32f63705cfcc57f44cd3531f57aa8defe388372242f5e",
	},
	// TODO: macOS 暂时保留 FFmpeg 6.1.1。Shaka 8.1.2 构建最低要求 macOS 15，
	// 无法兼容目前仍需支持的 macOS 12–14；其他兼容构建又会让发布包增大 20 MB
	// 以上。等 macOS 15 已足够老、可作为项目最低支持版本时再升级。
	"darwin/amd64": {
		version:      "6.1.1",
		url:          "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-x64.gz",
		downloadSHA:  "929b375c1182d956c51f7ac25e0b2b0411fb01f6f407aa15c9758efeb4242106",
		binarySHA256: "ebdddc936f61e14049a2d4b549a412b8a40deeff6540e58a9f2a2da9e6b18894",
	},
	"darwin/arm64": {
		version:      "6.1.1",
		url:          "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-arm64.gz",
		downloadSHA:  "8923876afa8db5585022d7860ec7e589af192f441c56793971276d450ed3bbfa",
		binarySHA256: "a90e3db6a3fd35f6074b013f948b1aa45b31c6375489d39e572bea3f18336584",
	},
}

// FFmpegToolStatus describes the project-local FFmpeg installation.
type FFmpegToolStatus struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	Supported        bool   `json:"supported"`
	Installed        bool   `json:"installed"`
	UpgradeAvailable bool   `json:"upgrade_available"`
	Source           string `json:"source,omitempty"`
	Downloading      bool   `json:"downloading"`
	Progress         int    `json:"progress"`
	DownloadedBytes  int64  `json:"downloaded_bytes"`
	TotalBytes       int64  `json:"total_bytes"`
	Path             string `json:"path"`
	Error            string `json:"error,omitempty"`
}

// FFmpegToolManager downloads FFmpeg into the running project's persistent data/tools directory.
type FFmpegToolManager struct {
	mu sync.Mutex

	context     context.Context
	targetPath  string
	displayPath string
	bundledDir  string
	tempDir     string
	version     string
	downloadURL string
	downloadSHA string
	binarySHA   string
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
	manager := &FFmpegToolManager{
		context:       ctx,
		targetPath:    filepath.Join(baseDir, relativePath),
		displayPath:   filepath.ToSlash(relativePath),
		bundledDir:    filepath.Join(baseDir, "internal", "bin"),
		tempDir:       os.TempDir(),
		version:       download.version,
		downloadURL:   download.url,
		downloadSHA:   download.downloadSHA,
		binarySHA:     download.binarySHA256,
		httpClient:    util.NewHTTPClient(0),
		resolveFFmpeg: util.ResolveFFmpegPath,
		containerMode: runtimeconfig.ContainerMode(),
	}
	if manager.containerMode {
		manager.displayPath = util.ContainerFFBinaryDir + "/ffmpeg"
		manager.bundledDir = util.ContainerFFBinaryDir
		manager.downloadURL = ""
		return manager
	}
	if manager.managedFFmpegNeedsUpgrade() {
		logging.Info("managed FFmpeg at %s does not match release %s and can be upgraded", manager.displayPath, manager.version)
	}
	return manager
}

// Status returns a snapshot of the current installation and download state.
func (m *FFmpegToolManager) Status() FFmpegToolStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, source, upgradeAvailable := m.detectInstallation()
	version := ""
	if source == "downloaded" {
		version = m.version
	}
	return FFmpegToolStatus{
		Name:             "ffmpeg",
		Version:          version,
		Supported:        m.downloadURL != "",
		Installed:        installed,
		UpgradeAvailable: upgradeAvailable,
		Source:           source,
		Downloading:      m.downloading,
		Progress:         m.progress,
		DownloadedBytes:  m.downloadedBytes,
		TotalBytes:       m.totalBytes,
		Path:             m.displayPath,
		Error:            m.downloadError,
	}
}

// StartDownload starts an asynchronous download. It returns false when FFmpeg
// is already installed or a download is already running.
func (m *FFmpegToolManager) StartDownload() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, _, _ := m.detectInstallation()
	if m.downloading || installed {
		return false, nil
	}
	if m.containerMode {
		return false, errors.New("FFmpeg must be provided by the Docker image at /app/internal/bin/ffmpeg; rebuild the image to restore it")
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

func (m *FFmpegToolManager) detectInstallation() (bool, string, bool) {
	if m.containerMode {
		if m.resolveFFmpeg != nil {
			resolvedPath, err := m.resolveFFmpeg()
			if err == nil && resolvedPath == util.ContainerFFBinaryDir+"/ffmpeg" {
				return true, "builtin", false
			}
		}
		return false, "", false
	}
	if m.resolveFFmpeg != nil {
		if resolvedPath, err := m.resolveFFmpeg(); err == nil && isUsableFFmpegFile(resolvedPath) {
			switch {
			case sameFilePath(resolvedPath, m.targetPath):
				if m.managedFFmpegIsCurrent() {
					return true, "downloaded", false
				}
				return false, "", true
			case pathWithinDirectory(resolvedPath, m.bundledDir):
				return true, "builtin", false
			}
		}
	}
	if m.managedFFmpegIsCurrent() {
		return true, "downloaded", false
	}
	return false, "", m.managedFFmpegNeedsUpgrade()
}

func (m *FFmpegToolManager) managedFFmpegIsCurrent() bool {
	if !isUsableFFmpegFile(m.targetPath) {
		return false
	}
	if strings.TrimSpace(m.binarySHA) == "" {
		return true
	}
	return fileMatchesSHA256(m.targetPath, m.binarySHA)
}

func (m *FFmpegToolManager) managedFFmpegNeedsUpgrade() bool {
	return isUsableFFmpegFile(m.targetPath) && !m.managedFFmpegIsCurrent()
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
	req.Header.Set("User-Agent", "JavBoss/"+m.version)

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

	archiveFile, err := os.CreateTemp(m.tempDir, ".ffmpeg-archive-*")
	if err != nil {
		return fmt.Errorf("create FFmpeg archive file: %w", err)
	}
	defer os.Remove(archiveFile.Name())
	defer archiveFile.Close()

	hasher := sha256.New()
	countedBody := &downloadProgressReader{
		reader: resp.Body,
		update: m.updateProgress,
		hash:   hasher,
	}
	if _, err := io.Copy(archiveFile, countedBody); err != nil {
		return fmt.Errorf("save FFmpeg download: %w", err)
	}
	if actualSHA := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(actualSHA, m.downloadSHA) {
		return fmt.Errorf("verify FFmpeg download checksum: got %s", actualSHA)
	}
	if _, err := archiveFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind FFmpeg download: %w", err)
	}

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

	extractErr := extractFFmpegDownload(archiveFile, tempFile, filepath.Base(m.targetPath))
	closeErr := tempFile.Close()
	if extractErr != nil {
		return fmt.Errorf("extract FFmpeg: %w", extractErr)
	}
	if closeErr != nil {
		return fmt.Errorf("save FFmpeg: %w", closeErr)
	}
	if !fileMatchesSHA256(tempPath, m.binarySHA) {
		return errors.New("verify FFmpeg binary checksum: mismatch")
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
	if m.managedFFmpegIsCurrent() {
		return nil
	}
	if err := installFFmpegFile(tempPath, m.targetPath); err != nil {
		return fmt.Errorf("install FFmpeg: %w", err)
	}
	return nil
}

func installFFmpegFile(sourcePath string, targetPath string) error {
	return installFFmpegFileWithRename(sourcePath, targetPath, replaceFileAtomic)
}

func installFFmpegFileWithRename(
	sourcePath string,
	targetPath string,
	renameFile func(string, string) error,
) error {
	stagedPath, err := copyFFmpegToTargetDirectory(sourcePath, targetPath)
	if err != nil {
		return fmt.Errorf("stage FFmpeg in target directory: %w", err)
	}
	defer os.Remove(stagedPath)

	if err := renameFile(stagedPath, targetPath); err != nil {
		return fmt.Errorf("atomically replace FFmpeg: %w", err)
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

func fileMatchesSHA256(path string, expectedSHA string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), strings.TrimSpace(expectedSHA))
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
