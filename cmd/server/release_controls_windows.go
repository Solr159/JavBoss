package main

import (
	"context"
	"log"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func runReleaseControls(ctx context.Context, stop context.CancelFunc, url, remoteURL string, logger *log.Logger) {
	// The hidden window and its Windows message loop must share an OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	runReleaseTray(ctx, stop, url, remoteURL, logger)
}

func notifyAlreadyRunning(message string) {
	// GUI releases have no stdin; never leave a duplicate process waiting on it.
	text, _ := windows.UTF16PtrFromString(message)
	title, _ := windows.UTF16PtrFromString("JavBoss")
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(
		0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x40,
	)
}
