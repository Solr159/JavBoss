package mpv

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var (
	focusRestoreMu         sync.Mutex
	focusRestoreScriptPath string
	focusRestoreTargetPath string
)

func buildFocusRestoreCommand() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	scriptPath, err := ensureFocusRestoreScript()
	if err != nil {
		return ""
	}
	return "run /bin/sh " + quoteMPVInputArg(scriptPath)
}

func writeDarwinFocusRestoreTarget(pid int, bundleID string) error {
	if runtime.GOOS != "darwin" || pid <= 0 {
		return nil
	}

	focusRestoreMu.Lock()
	defer focusRestoreMu.Unlock()

	targetPath, err := ensureFocusRestoreTargetPathLocked()
	if err != nil {
		return err
	}

	content := strings.Join([]string{
		strings.TrimSpace(bundleID),
		strconv.Itoa(pid),
	}, "\n") + "\n"
	return os.WriteFile(targetPath, []byte(content), 0o600)
}

func ensureFocusRestoreScript() (string, error) {
	focusRestoreMu.Lock()
	defer focusRestoreMu.Unlock()

	if focusRestoreScriptPath == "" {
		path, err := sessionPath("mpv-restore-focus.sh")
		if err != nil {
			return "", err
		}
		focusRestoreScriptPath = path
	}
	if _, err := os.Stat(focusRestoreScriptPath); err == nil {
		return focusRestoreScriptPath, nil
	}

	targetPath, err := ensureFocusRestoreTargetPathLocked()
	if err != nil {
		return "", err
	}
	script := fmt.Sprintf(`#!/bin/sh
set -u
target_file=%s
[ -f "$target_file" ] || exit 0
bundle_id="$(sed -n '1p' "$target_file" 2>/dev/null | tr -d '\r\n')"
if [ -n "$bundle_id" ]; then
  /usr/bin/open -b "$bundle_id" >/dev/null 2>&1 && exit 0
  /usr/bin/osascript -e "tell application id \"$bundle_id\" to activate" >/dev/null 2>&1 && exit 0
fi
exit 0
`, shellSingleQuote(targetPath))
	if err := os.WriteFile(focusRestoreScriptPath, []byte(script), 0o755); err != nil {
		return "", err
	}
	return focusRestoreScriptPath, nil
}

func ensureFocusRestoreTargetPathLocked() (string, error) {
	if focusRestoreTargetPath != "" {
		return focusRestoreTargetPath, nil
	}
	path, err := sessionPath("mpv-restore-focus-target")
	if err != nil {
		return "", err
	}
	focusRestoreTargetPath = path
	return focusRestoreTargetPath, nil
}

func quoteMPVInputArg(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
