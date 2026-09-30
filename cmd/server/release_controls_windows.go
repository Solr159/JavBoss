package main

import (
	"context"
	"log"
	"runtime"
	"unsafe"

	"javboss/assets/branding"
	"javboss/internal/util"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
)

func runReleaseControls(ctx context.Context, stop context.CancelFunc, url, remoteURL string, logger *log.Logger) {
	// The hidden window and its Windows message loop must share an OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	systray.Run(func() {
		systray.SetIcon(branding.Icon)
		tooltip := "JavBoss — " + url
		if remoteURL != "" {
			tooltip = "JavBoss (Client) — " + url
		}
		systray.SetTooltip(tooltip)
		openLabel, exitLabel := "Open new page", "Exit"
		if util.SystemPrefersChinese() {
			openLabel, exitLabel = "打开新页面", "退出程序"
		}
		openItem := systray.AddMenuItem(openLabel, url)
		systray.AddSeparator()
		exitItem := systray.AddMenuItem(exitLabel, "")
		// Return from onReady before handling clicks: systray waits for this
		// callback to finish before displaying the menu.
		go func() {
			if ctx.Err() == nil {
				openReleasePage(url, logger)
			}
			for {
				select {
				case <-ctx.Done():
					systray.Quit()
					return
				case <-openItem.ClickedCh:
					openReleasePage(url, logger)
				case <-exitItem.ClickedCh:
					stop()
				}
			}
		}()
	}, stop)
}

func notifyAlreadyRunning(message string) {
	// GUI releases have no stdin; never leave a duplicate process waiting on it.
	text, _ := windows.UTF16PtrFromString(message)
	title, _ := windows.UTF16PtrFromString("JavBoss")
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(
		0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x40,
	)
}
