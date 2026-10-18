package cover

import (
	"context"
	"sync/atomic"
)

var active atomic.Pointer[queue]

// Start configures and starts cover downloads. Call once before starting consumers.
// An empty directory disables background downloads.
func Start(ctx context.Context, coverDir string) {
	q := newQueue(coverDir)
	active.Store(q)
	q.Start(ctx)
}

// Enqueue schedules a cover download, or does nothing when downloads are disabled.
func Enqueue(code string) { active.Load().Enqueue(code) }

// Exists reports whether a valid cover is cached in the configured directory.
// Use FindPath to look in a directory without starting background downloads.
func Exists(code string) bool {
	q := active.Load()
	return q != nil && exists(q.coverDir, code)
}

// Enabled reports whether background cover downloads are configured.
func Enabled() bool { return active.Load() != nil }
