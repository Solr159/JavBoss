package mpv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"pornboss/internal/common/logging"
)

const playbackScreenshotTemplate = "mpv_%wH-%wM-%wS.%wT"

type PlayOptions struct {
	DataDir      string
	VideoID      int64
	StartTimeSec float64
}

// PlayVideo launches mpv to play the given file path.
func PlayVideo(path string, options PlayOptions) error {
	cmd, err := buildCommand(path, options)
	if err != nil {
		return err
	}
	if err := startCommand(cmd, "play video"); err != nil {
		return fmt.Errorf("play video: %w", err)
	}
	return nil
}

func buildCommand(path string, options PlayOptions) (*exec.Cmd, error) {
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
	args := make([]string, 0, 8)
	args = append(args, "--config-dir="+modernZ.ConfigDir)
	if runtime.GOOS == "linux" && os.Getenv("PORNBOSS_BUILD_MODE") != "release" {
		args = append(args, "--vo=x11")
	}
	args = append(args, "--load-scripts=no")
	args = append(args, "--include="+mpvConfigPath)
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
	args = append(args, "--", path)
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

func startCommand(cmd *exec.Cmd, label string) error {
	logging.Info("%s command: %v", label, cmd.Args)
	if err := cmd.Start(); err != nil {
		return err
	}
	focusStartedProcessWindow(cmd.Process.Pid, label)
	go func() {
		if err := cmd.Wait(); err != nil {
			logging.Error("%s command exited with error: %v", label, err)
		}
	}()
	return nil
}
