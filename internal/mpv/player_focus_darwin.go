//go:build darwin

package mpv

/*
#cgo darwin LDFLAGS: -framework AppKit -framework CoreGraphics -framework ApplicationServices
#import <stdbool.h>
#import <stdlib.h>
#import <sys/types.h>

bool javbossProcessHasWindow(pid_t pid);
bool javbossActivateProcess(pid_t pid);
pid_t javbossFrontmostProcessID(void);
char* javbossFrontmostBundleID(void);
*/
import "C"

import (
	"time"
	"unsafe"

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

func rememberFocusRestoreOwner(excludePID int) {
	rememberDarwinFocusOwner(excludePID)
}

func activateDarwinProcessWindow(pid int, timeout time.Duration) bool {
	rememberDarwinFocusOwner(pid)

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

func rememberDarwinFocusOwner(excludePID int) {
	frontmostPID := int(C.javbossFrontmostProcessID())
	if frontmostPID <= 0 || frontmostPID == excludePID {
		return
	}

	var bundleID string
	bundleIDCString := C.javbossFrontmostBundleID()
	if bundleIDCString != nil {
		bundleID = C.GoString(bundleIDCString)
		C.free(unsafe.Pointer(bundleIDCString))
	}

	if err := writeDarwinFocusRestoreTarget(frontmostPID, bundleID); err != nil {
		logging.Error("write macOS mpv focus restore target failed: %v", err)
	}
}
