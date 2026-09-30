//go:build windows || darwin

package main

import (
	"context"
	"log"

	"javboss/assets/branding"
	"javboss/internal/util"

	"fyne.io/systray"
)

func runReleaseTray(ctx context.Context, stop context.CancelFunc, url, remoteURL string, logger *log.Logger) {
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
