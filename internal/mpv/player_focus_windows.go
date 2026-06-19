//go:build windows

package mpv

import (
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"javboss/internal/common/logging"
)

const (
	focusWindowPollInterval = 50 * time.Millisecond
	focusWindowTimeout      = 3 * time.Second

	gwOwner      = 4
	swRestore    = 9
	waActive     = 1
	wmActivate   = 0x0006
	wmNCActivate = 0x0086
)

var (
	moduser32   = windows.NewLazySystemDLL("user32.dll")
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procAttachThreadInput      = moduser32.NewProc("AttachThreadInput")
	procBringWindowToTop       = moduser32.NewProc("BringWindowToTop")
	procEnumWindows            = moduser32.NewProc("EnumWindows")
	procGetForegroundWindow    = moduser32.NewProc("GetForegroundWindow")
	procGetWindow              = moduser32.NewProc("GetWindow")
	procGetWindowThreadProcess = moduser32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible        = moduser32.NewProc("IsWindowVisible")
	procSendMessage            = moduser32.NewProc("SendMessageW")
	procSetActiveWindow        = moduser32.NewProc("SetActiveWindow")
	procSetFocus               = moduser32.NewProc("SetFocus")
	procSetForegroundWindow    = moduser32.NewProc("SetForegroundWindow")
	procShowWindow             = moduser32.NewProc("ShowWindow")
	procGetCurrentThreadID     = modkernel32.NewProc("GetCurrentThreadId")

	enumWindowsFindProcessWindowCallback = windows.NewCallback(enumWindowsFindProcessWindow)
)

type findProcessWindowState struct {
	pid   uint32
	found windows.Handle
}

func focusStartedProcessWindow(pid int, label string) {
	if pid <= 0 {
		return
	}
	go func() {
		hwnd := waitForProcessWindow(uint32(pid), focusWindowTimeout)
		if hwnd == 0 {
			logging.Info("%s window focus skipped: mpv window not found for pid %d", label, pid)
			return
		}
		if !activateWindow(hwnd) {
			logging.Info("%s window focus request was not accepted by Windows for pid %d", label, pid)
		}
	}()
}

func waitForProcessWindow(pid uint32, timeout time.Duration) windows.Handle {
	deadline := time.Now().Add(timeout)
	for {
		if hwnd := findProcessWindow(pid); hwnd != 0 {
			return hwnd
		}
		if time.Now().After(deadline) {
			return 0
		}
		time.Sleep(focusWindowPollInterval)
	}
}

func findProcessWindow(pid uint32) windows.Handle {
	state := findProcessWindowState{pid: pid}
	procEnumWindows.Call(enumWindowsFindProcessWindowCallback, uintptr(unsafe.Pointer(&state)))
	runtime.KeepAlive(&state)
	return state.found
}

func enumWindowsFindProcessWindow(hwnd windows.Handle, lparam uintptr) uintptr {
	state := (*findProcessWindowState)(unsafe.Pointer(lparam))
	if state == nil {
		return 0
	}
	if windowProcessID(hwnd) == state.pid && isUsableTopLevelWindow(hwnd) {
		state.found = hwnd
		return 0
	}
	return 1
}

func isUsableTopLevelWindow(hwnd windows.Handle) bool {
	if hwnd == 0 {
		return false
	}
	visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	if visible == 0 {
		return false
	}
	owner, _, _ := procGetWindow.Call(uintptr(hwnd), gwOwner)
	return owner == 0
}

func windowProcessID(hwnd windows.Handle) uint32 {
	var pid uint32
	procGetWindowThreadProcess.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pid)))
	return pid
}

func windowThreadID(hwnd windows.Handle) uint32 {
	threadID, _, _ := procGetWindowThreadProcess.Call(uintptr(hwnd), 0)
	return uint32(threadID)
}

func activateWindow(hwnd windows.Handle) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	procShowWindow.Call(uintptr(hwnd), swRestore)

	currentThreadID := getCurrentThreadID()
	foregroundThreadID := windowThreadID(getForegroundWindow())
	targetThreadID := windowThreadID(hwnd)

	attachedForeground := attachThreadInput(currentThreadID, foregroundThreadID, true)
	attachedTarget := attachThreadInput(currentThreadID, targetThreadID, true)
	defer func() {
		if attachedTarget {
			attachThreadInput(currentThreadID, targetThreadID, false)
		}
		if attachedForeground {
			attachThreadInput(currentThreadID, foregroundThreadID, false)
		}
	}()

	procBringWindowToTop.Call(uintptr(hwnd))
	procSetActiveWindow.Call(uintptr(hwnd))
	procSetFocus.Call(uintptr(hwnd))
	ret, _, _ := procSetForegroundWindow.Call(uintptr(hwnd))
	syncWindowActivation(hwnd)
	return ret != 0 || getForegroundWindow() == hwnd
}

func syncWindowActivation(hwnd windows.Handle) {
	procSendMessage.Call(uintptr(hwnd), wmNCActivate, 1, 0)
	procSendMessage.Call(uintptr(hwnd), wmActivate, waActive, 0)
}

func getForegroundWindow() windows.Handle {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return windows.Handle(hwnd)
}

func getCurrentThreadID() uint32 {
	threadID, _, _ := procGetCurrentThreadID.Call()
	return uint32(threadID)
}

func attachThreadInput(sourceThreadID, targetThreadID uint32, attach bool) bool {
	if sourceThreadID == 0 || targetThreadID == 0 || sourceThreadID == targetThreadID {
		return false
	}
	ret, _, _ := procAttachThreadInput.Call(
		uintptr(sourceThreadID),
		uintptr(targetThreadID),
		boolToUintptr(attach),
	)
	return ret != 0
}

func boolToUintptr(value bool) uintptr {
	if value {
		return 1
	}
	return 0
}
