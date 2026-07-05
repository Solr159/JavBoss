package mpv

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	focusRestoreMu         sync.Mutex
	focusRestoreScriptPath string
	focusRestoreTargetPath string
	focusRestoreCancelPath string
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

func cancelFocusRestoreAttempts() {
	if runtime.GOOS != "darwin" {
		return
	}

	focusRestoreMu.Lock()
	defer focusRestoreMu.Unlock()

	cancelPath, err := ensureFocusRestoreCancelPathLocked()
	if err != nil {
		return
	}
	token := strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.WriteFile(cancelPath, []byte(token+"\n"), 0o600); err != nil {
		return
	}
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

	targetPath, err := ensureFocusRestoreTargetPathLocked()
	if err != nil {
		return "", err
	}
	cancelPath, err := ensureFocusRestoreCancelPathLocked()
	if err != nil {
		return "", err
	}
	script := fmt.Sprintf(`#!/bin/sh
set -u
target_file=%s
cancel_file=%s

read_cancel_token() {
  if [ -f "$cancel_file" ]; then
    sed -n '1p' "$cancel_file" 2>/dev/null | tr -d '\r\n'
  fi
}

initial_cancel_token="$(read_cancel_token)"

check_cancelled() {
  current_cancel_token="$(read_cancel_token)"
  if [ "$current_cancel_token" != "$initial_cancel_token" ]; then
    exit 0
  fi
}

if [ ! -f "$target_file" ]; then
  exit 0
fi
bundle_id="$(sed -n '1p' "$target_file" 2>/dev/null | tr -d '\r\n')"
target_pid="$(sed -n '2p' "$target_file" 2>/dev/null | tr -cd '0-9')"

activate_with_open() {
  [ -n "$bundle_id" ] || return 1
  /usr/bin/open -b "$bundle_id" >/dev/null 2>&1
}

activate_with_osascript() {
  /usr/bin/osascript - "$bundle_id" "${target_pid:-0}" <<'APPLESCRIPT' >/dev/null 2>&1
on run argv
  set bundleID to item 1 of argv
  set targetPIDText to item 2 of argv
  set targetPID to 0
  try
    set targetPID to targetPIDText as integer
  end try

  if bundleID is not "" then
    try
      tell application id bundleID to activate
      return
    end try
  end if

  if targetPID > 0 then
    try
      tell application "System Events"
        repeat with proc in processes
          try
            if unix id of proc is targetPID then
              set frontmost of proc to true
              return
            end if
          end try
        end repeat
      end tell
    end try
  end if
end run
APPLESCRIPT
}

# mpv may briefly reclaim focus while it processes stop/minimize. Retry activation
# across that window instead of exiting after the first successful request.
for delay in 0.05 0.12 0.25 0.45 0.70; do
  check_cancelled
  sleep "$delay"
  check_cancelled
  activate_with_open
  activate_with_osascript
done
exit 0
`, shellSingleQuote(targetPath), shellSingleQuote(cancelPath))
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

func ensureFocusRestoreCancelPathLocked() (string, error) {
	if focusRestoreCancelPath != "" {
		return focusRestoreCancelPath, nil
	}
	path, err := sessionPath("mpv-restore-focus-cancel")
	if err != nil {
		return "", err
	}
	focusRestoreCancelPath = path
	return focusRestoreCancelPath, nil
}

func quoteMPVInputArg(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
