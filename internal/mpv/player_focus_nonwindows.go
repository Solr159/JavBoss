//go:build !windows && !darwin

package mpv

func focusStartedProcessWindow(pid int, label string) {}

func rememberFocusRestoreOwner(pid int) {}
