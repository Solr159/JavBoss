package directory

import (
	"context"
	"sync/atomic"
	"time"

	"javboss/internal/models"
)

var applicationTasks atomic.Pointer[Manager]

func init() { Init("") }

// Init configures directory tasks before consumers start. Previous tasks must have stopped.
func Init(coverDir string) { applicationTasks.Store(New(Options{CoverDir: coverDir})) }

func DirectoryWorkSnapshot(id int64) (string, *DirectoryScanProgress) {
	return applicationTasks.Load().DirectoryWorkSnapshot(id)
}

func DirectoryWorkStatus(id int64) string { return applicationTasks.Load().DirectoryWorkStatus(id) }

func IsDirectoryScanning(id int64) bool { return applicationTasks.Load().IsDirectoryScanning(id) }

func ScanDirectory(ctx context.Context, dir models.Directory) (*Summary, error) {
	return applicationTasks.Load().ScanDirectory(ctx, dir)
}

func StartManualDirectoryScan(dir models.Directory) error {
	return applicationTasks.Load().StartManualDirectoryScan(dir)
}

func CancelAndReserveDirectoryScan(ctx context.Context, id int64) (func(), error) {
	return applicationTasks.Load().CancelAndReserveDirectoryScan(ctx, id)
}

func StartDirectoryProcessing(ctx context.Context, dir models.Directory, mode, layout string) error {
	return applicationTasks.Load().StartDirectoryProcessing(ctx, dir, mode, layout)
}

func StartAutomaticDirectoryScanScheduler(ctx context.Context, interval time.Duration) {
	applicationTasks.Load().StartAutomaticDirectoryScanScheduler(ctx, interval)
}
