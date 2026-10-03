//go:build !windows

package util

import "context"

// BackgroundCombinedOutput runs a background tool and captures stdout and stderr.
func BackgroundCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return BackgroundCommandContext(ctx, name, args...).CombinedOutput()
}
