package mpv

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
)

var (
	focusRestoreMu         sync.Mutex
	focusRestoreScriptPath string
	focusRestoreTargetPath string
	focusRestoreLogPath    string
	focusRestoreCancelPath string
)

func buildFocusRestoreCommand() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	scriptPath, err := ensureFocusRestoreScript()
	if err != nil {
		logging.Error("ensure macOS mpv focus restore script failed: %v", err)
		return ""
	}
	logPath, _ := ensureFocusRestoreLogPath()
	if logPath != "" {
		logging.Info("macOS mpv focus restore script ready: script=%s log=%s", scriptPath, logPath)
	} else {
		logging.Info("macOS mpv focus restore script ready: script=%s", scriptPath)
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
	logging.Info("macOS mpv focus restore target: pid=%d bundle_id=%q target=%s", pid, strings.TrimSpace(bundleID), targetPath)
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
		logging.Error("resolve macOS mpv focus restore cancel path failed: %v", err)
		return
	}
	token := strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.WriteFile(cancelPath, []byte(token+"\n"), 0o600); err != nil {
		logging.Error("write macOS mpv focus restore cancel token failed: %v", err)
		return
	}
	logging.Info("macOS mpv focus restore attempts canceled: token=%s cancel=%s", token, cancelPath)
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
	logPath, err := ensureFocusRestoreLogPathLocked()
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
log_file=%s
cancel_file=%s

log() {
  printf '%%s %%s\n' "$(/bin/date '+%%Y-%%m-%%d %%H:%%M:%%S')" "$*" >> "$log_file"
}

run_and_log() {
  label="$1"
  shift
  output="$("$@" 2>&1)"
  status=$?
  log "$label status=$status output=$output"
  return "$status"
}

read_cancel_token() {
  if [ -f "$cancel_file" ]; then
    sed -n '1p' "$cancel_file" 2>/dev/null | tr -d '\r\n'
  fi
}

initial_cancel_token="$(read_cancel_token)"

check_cancelled() {
  current_cancel_token="$(read_cancel_token)"
  if [ "$current_cancel_token" != "$initial_cancel_token" ]; then
    log "cancelled initial_token=$initial_cancel_token current_token=$current_cancel_token"
    exit 0
  fi
}

log "start pid=$$"
log "target_file=$target_file"
log "cancel_file=$cancel_file token=$initial_cancel_token"
if [ ! -f "$target_file" ]; then
  log "target file missing"
  exit 0
fi
bundle_id="$(sed -n '1p' "$target_file" 2>/dev/null | tr -d '\r\n')"
target_pid="$(sed -n '2p' "$target_file" 2>/dev/null | tr -cd '0-9')"
log "target bundle_id=$bundle_id pid=${target_pid:-0}"

activate_with_open() {
  [ -n "$bundle_id" ] || return 1
  run_and_log "open bundle" /usr/bin/open -b "$bundle_id"
}

activate_with_osascript() {
  run_and_log "osascript activate" /usr/bin/osascript - "$bundle_id" "${target_pid:-0}" <<'APPLESCRIPT'
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
  log "sleep $delay"
  sleep "$delay"
  check_cancelled
  activate_with_open
  activate_with_osascript
done
log "done"
exit 0
`, shellSingleQuote(targetPath), shellSingleQuote(logPath), shellSingleQuote(cancelPath))
	if err := os.WriteFile(focusRestoreScriptPath, []byte(script), 0o755); err != nil {
		return "", err
	}
	logging.Info("wrote macOS mpv focus restore script: script=%s target=%s log=%s", focusRestoreScriptPath, targetPath, logPath)
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

func ensureFocusRestoreLogPath() (string, error) {
	focusRestoreMu.Lock()
	defer focusRestoreMu.Unlock()
	return ensureFocusRestoreLogPathLocked()
}

func ensureFocusRestoreLogPathLocked() (string, error) {
	if focusRestoreLogPath != "" {
		return focusRestoreLogPath, nil
	}
	path, err := sessionPath("mpv-restore-focus.log")
	if err != nil {
		return "", err
	}
	focusRestoreLogPath = path
	return focusRestoreLogPath, nil
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
