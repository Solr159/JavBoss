package mpv

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"javboss/internal/common/logging"
)

const playbackScreenshotTemplate = "mpv_%wH-%wM-%wS.%wT"

const (
	ipcCommandTimeout = 5 * time.Second
	ipcReadyTimeout   = 5 * time.Second
)

type PlayOptions struct {
	DataDir      string
	VideoID      int64
	StartTimeSec float64
}

type playerSession struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	ipcPath string
}

type ipcRequest struct {
	Command   []any `json:"command"`
	RequestID int64 `json:"request_id"`
}

type ipcResponse struct {
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	RequestID int64           `json:"request_id"`
}

type ipcResponseError struct {
	command string
	message string
}

func (e *ipcResponseError) Error() string {
	return fmt.Sprintf("mpv ipc command %s failed: %s", e.command, e.message)
}

var (
	defaultSession     playerSession
	nextIPCRequestID   atomic.Int64
	dialMPVIPCOverride func(path string, timeout time.Duration) (io.ReadWriteCloser, error)
)

// PlayVideo launches mpv to play the given file path.
func PlayVideo(path string, options PlayOptions) error {
	if !loadConfiguredPlayerReuseWindow() {
		return playVideoInNewProcess(path, options)
	}
	return defaultSession.PlayVideo(path, options)
}

func ResetPlayerSession() {
	defaultSession.Reset()
}

func playVideoInNewProcess(path string, options PlayOptions) error {
	cmd, err := buildOneShotCommand(path, options)
	if err != nil {
		return err
	}
	logging.Info("play video command: %v", cmd.Args)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("play video: %w", err)
	}
	focusStartedProcessWindow(cmd.Process.Pid, "play video")
	go func() {
		if err := cmd.Wait(); err != nil {
			logging.Error("play video command exited with error: %v", err)
		}
	}()
	return nil
}

func (s *playerSession) PlayVideo(path string, options PlayOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := s.ensureRunningLocked(); err != nil {
			return err
		}
		if err := s.playVideoLocked(path, options); err != nil {
			lastErr = err
			if isIPCResponseError(err) {
				return err
			}
			logging.Error("mpv ipc playback failed, restarting player: %v", err)
			s.stopLocked()
			continue
		}
		if s.cmd != nil && s.cmd.Process != nil {
			focusStartedProcessWindow(s.cmd.Process.Pid, "play video")
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("play video: %w", lastErr)
	}
	return errors.New("play video failed")
}

func (s *playerSession) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopLocked()
}

func (s *playerSession) ensureRunningLocked() error {
	if s.cmd != nil && s.cmd.ProcessState == nil && s.ipcPath != "" {
		return nil
	}

	cmd, ipcPath, err := buildCommandWithIPC("", PlayOptions{})
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Remove(ipcPath)
	}

	logging.Info("play video command: %v", cmd.Args)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("play video: %w", err)
	}

	s.cmd = cmd
	s.ipcPath = ipcPath
	focusStartedProcessWindow(cmd.Process.Pid, "play video")

	go s.waitForExit(cmd)

	if err := waitForIPCReady(ipcPath); err != nil {
		s.stopLocked()
		return err
	}
	return nil
}

func (s *playerSession) waitForExit(cmd *exec.Cmd) {
	if err := cmd.Wait(); err != nil {
		logging.Error("play video command exited with error: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == cmd {
		s.cmd = nil
		s.ipcPath = ""
	}
}

func (s *playerSession) stopLocked() {
	cmd := s.cmd
	ipcPath := s.ipcPath
	s.cmd = nil
	s.ipcPath = ""

	if cmd != nil && cmd.Process != nil && cmd.ProcessState == nil {
		_ = cmd.Process.Kill()
	}
	if runtime.GOOS != "windows" && ipcPath != "" {
		_ = os.Remove(ipcPath)
	}
}

func (s *playerSession) playVideoLocked(path string, options PlayOptions) error {
	commands, err := buildBeforeLoadCommands(options)
	if err != nil {
		return err
	}
	for _, command := range commands {
		if err := runIPCCommand(s.ipcPath, command); err != nil {
			if isIPCResponseError(err) && isOptionalBeforeLoadCommand(command) {
				logging.Error("optional mpv ipc command ignored: %v", err)
				continue
			}
			return err
		}
	}
	return runIPCCommand(s.ipcPath, buildLoadFileCommand(path, options))
}

func buildCommand(path string, options PlayOptions) (*exec.Cmd, error) {
	cmd, _, err := buildCommandWithIPC(path, options)
	return cmd, err
}

func buildOneShotCommand(path string, options PlayOptions) (*exec.Cmd, error) {
	return buildCommandArgs(path, options, "")
}

func buildCommandWithIPC(path string, options PlayOptions) (*exec.Cmd, string, error) {
	ipcPath, err := playbackIPCPath()
	if err != nil {
		return nil, "", err
	}
	cmd, err := buildCommandArgs(path, options, ipcPath)
	if err != nil {
		return nil, "", err
	}
	return cmd, ipcPath, nil
}

func buildCommandArgs(path string, options PlayOptions, ipcPath string) (*exec.Cmd, error) {
	mpvPath, err := ResolvePath()
	if err != nil {
		return nil, err
	}
	inputConfPath, err := ensureInputConf()
	if err != nil {
		return nil, err
	}
	mpvConfigPath, err := ensureConfig()
	if err != nil {
		return nil, err
	}
	modernZ, err := ensureModernZAssets()
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, 12)
	args = append(args, "--config-dir="+modernZ.ConfigDir)
	if runtime.GOOS == "linux" && os.Getenv("JAVBOSS_BUILD_MODE") != "release" {
		args = append(args, "--vo=x11")
	}
	args = append(args, "--load-scripts=no")
	args = append(args, "--include="+mpvConfigPath)
	if ipcPath != "" {
		args = append(args, "--idle=yes")
		args = append(args, "--force-window=yes")
		args = append(args, "--input-ipc-server="+ipcPath)
	}
	args = append(args, buildThumbfastScriptArgs(mpvPath)...)
	args = append(args, "--script="+modernZ.ScriptPath)
	args = append(args, "--script="+modernZ.ThumbfastScriptPath)
	if screenshotArgs, err := buildPlaybackScreenshotArgs(options); err != nil {
		return nil, err
	} else if len(screenshotArgs) > 0 {
		args = append(args, screenshotArgs...)
	}
	if hotkeyHint, err := buildStartupHotkeyHint(); err != nil {
		return nil, err
	} else if hotkeyHint != "" {
		args = append(args, "--osd-playing-msg="+hotkeyHint)
	}
	args = append(args, buildPlaybackStartArgs(options)...)
	args = append(args, "--input-conf="+inputConfPath)
	if strings.TrimSpace(path) != "" {
		args = append(args, "--", path)
	}
	return exec.Command(mpvPath, args...), nil
}

func buildThumbfastScriptArgs(mpvPath string) []string {
	if strings.TrimSpace(mpvPath) == "" {
		return nil
	}
	return []string{"--script-opt=thumbfast-mpv_path=" + mpvPath}
}

func buildPlaybackStartArgs(options PlayOptions) []string {
	if options.StartTimeSec <= 0 {
		return nil
	}
	return []string{"--start=" + strconv.FormatFloat(options.StartTimeSec, 'f', -1, 64)}
}

func playbackIPCPath() (string, error) {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\javboss-mpv-` + strconv.Itoa(os.Getpid()), nil
	}
	return sessionPath("mpv-ipc.sock")
}

func buildBeforeLoadCommands(options PlayOptions) ([][]any, error) {
	commands := make([][]any, 0, 5)
	if loadConfiguredPlayerResumePlayback() {
		commands = append(commands, []any{"write-watch-later-config"})
	}
	commands = append(commands,
		[]any{"set_property", "window-minimized", false},
		[]any{"set_property", "pause", false},
		[]any{"set_property", "screenshot-template", playbackScreenshotTemplate},
	)

	dir, err := ensurePlaybackScreenshotDir(options)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		dir, err = ensureFallbackScreenshotDir()
		if err != nil {
			return nil, err
		}
	}
	if dir != "" {
		commands = append(commands, []any{"set_property", "screenshot-directory", dir})
	}
	return commands, nil
}

func isOptionalBeforeLoadCommand(command []any) bool {
	if len(command) == 0 {
		return false
	}
	name, _ := command[0].(string)
	if name == "write-watch-later-config" {
		return true
	}
	if len(command) < 2 {
		return false
	}
	property, _ := command[1].(string)
	return name == "set_property" && property == "window-minimized"
}

func buildLoadFileCommand(path string, options PlayOptions) []any {
	command := []any{"loadfile", path, "replace"}
	if options.StartTimeSec > 0 {
		command = append(command, "start="+strconv.FormatFloat(options.StartTimeSec, 'f', -1, 64))
	}
	return command
}

func buildPlaybackScreenshotArgs(options PlayOptions) ([]string, error) {
	screenshotDir, err := ensurePlaybackScreenshotDir(options)
	if err != nil {
		return nil, err
	}
	if screenshotDir == "" {
		return nil, nil
	}
	return []string{
		"--screenshot-directory=" + screenshotDir,
		"--screenshot-template=" + playbackScreenshotTemplate,
	}, nil
}

func ensurePlaybackScreenshotDir(options PlayOptions) (string, error) {
	dataDir := strings.TrimSpace(options.DataDir)
	if dataDir == "" || options.VideoID <= 0 {
		return "", nil
	}

	dir := filepath.Join(dataDir, "video", strconv.FormatInt(options.VideoID, 10), "screenshot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create mpv screenshot directory: %w", err)
	}
	return dir, nil
}

func ensureFallbackScreenshotDir() (string, error) {
	dir, err := sessionPath("screenshot")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create fallback mpv screenshot directory: %w", err)
	}
	return dir, nil
}

func waitForIPCReady(ipcPath string) error {
	deadline := time.Now().Add(ipcReadyTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := runIPCCommand(ipcPath, []any{"get_property", "pid"}); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("wait for mpv ipc: %w", lastErr)
	}
	return errors.New("wait for mpv ipc timed out")
}

func runIPCCommand(ipcPath string, command []any) error {
	conn, err := dialMPVIPC(ipcPath, ipcCommandTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	if deadlineConn, ok := conn.(interface{ SetDeadline(time.Time) error }); ok {
		_ = deadlineConn.SetDeadline(time.Now().Add(ipcCommandTimeout))
	}

	requestID := nextIPCRequestID.Add(1)
	raw, err := json.Marshal(ipcRequest{
		Command:   command,
		RequestID: requestID,
	})
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if _, err := conn.Write(raw); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}

		var response ipcResponse
		if err := json.Unmarshal(line, &response); err != nil {
			continue
		}
		if response.RequestID != requestID {
			continue
		}
		if response.Error != "" && response.Error != "success" {
			return &ipcResponseError{
				command: commandName(command),
				message: response.Error,
			}
		}
		return nil
	}
}

func commandName(command []any) string {
	if len(command) == 0 {
		return ""
	}
	name, _ := command[0].(string)
	return name
}

func isIPCResponseError(err error) bool {
	var responseErr *ipcResponseError
	return errors.As(err, &responseErr)
}

func dialMPVIPC(path string, timeout time.Duration) (io.ReadWriteCloser, error) {
	if dialMPVIPCOverride != nil {
		return dialMPVIPCOverride(path, timeout)
	}
	return dialPlatformMPVIPC(path, timeout)
}
