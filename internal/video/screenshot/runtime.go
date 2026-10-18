package screenshot

import (
	"context"
	"sync/atomic"

	"javboss/internal/db"
	"javboss/internal/models"
)

var active atomic.Pointer[queue]

// Start configures and starts thumbnail generation. Call once before consumers.
// An empty directory disables the queue; CaptureFile remains available.
func Start(ctx context.Context, dataDir string) {
	q := newQueue(dataDir, func(ctx context.Context, id int64) (*models.Video, error) {
		return db.Default().GetVideo(ctx, id)
	})
	active.Store(q)
	q.Start(ctx)
}

// GetThumbnail reads a cached thumbnail or waits for the shared queue to generate it.
func GetThumbnail(ctx context.Context, video *models.Video) (string, error) {
	return active.Load().GetThumbnail(ctx, video)
}

// EnqueueThumbnail schedules generation, or does nothing when the queue is disabled.
func EnqueueThumbnail(video *models.Video) { active.Load().EnqueueThumbnail(video) }
