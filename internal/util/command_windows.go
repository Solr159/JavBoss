package util

import (
	"context"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// BackgroundCommandContext creates a command for a noninteractive background tool.
// GUI releases have no console to inherit, so console tools need CREATE_NO_WINDOW
// even when their standard output and error are redirected to pipes.
func BackgroundCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}
