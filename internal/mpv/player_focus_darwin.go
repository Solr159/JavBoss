//go:build darwin

package mpv

/*
#cgo darwin LDFLAGS: -framework AppKit -framework CoreGraphics -framework ApplicationServices
#import <stdbool.h>
#import <sys/types.h>

bool javbossProcessHasWindow(pid_t pid);
bool javbossActivateProcess(pid_t pid);
*/
import "C"

import (
	"time"

	"javboss/internal/common/logging"
)

const (
	darwinFocusWindowPollInterval = 50 * time.Millisecond
	darwinFocusWindowTimeout      = 3 * time.Second
)

func focusStartedProcessWindow(pid int, label string) {
	if pid <= 0 {
		return
	}
	go func() {
		if !activateDarwinProcessWindow(pid, darwinFocusWindowTimeout) {
			logging.Info("%s window focus request was not accepted by macOS for pid %d", label, pid)
		}
	}()
}

func activateDarwinProcessWindow(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		hasWindow := bool(C.javbossProcessHasWindow(C.pid_t(pid)))
		activated := bool(C.javbossActivateProcess(C.pid_t(pid)))
		if hasWindow && activated {
			return true
		}
		if time.Now().After(deadline) {
			return activated
		}
		time.Sleep(darwinFocusWindowPollInterval)
	}
}
