//go:build !windows

package util

import (
	"context"
	"os/exec"
)

// BackgroundCommandContext creates a command for a noninteractive background tool.
func BackgroundCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
