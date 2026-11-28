package api

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	dbpkg "javboss/internal/db"
	"javboss/internal/mpv"
	"javboss/internal/util"
	"javboss/internal/video/screenshot"
)

func localScreenshotDeleter(dataDir string, videoID int64) mpv.ScreenshotDeleter {
	return func(ctx context.Context, second float64) error {
		names, err := screenshot.PlaybackNamesAt(dataDir, videoID, second)
		if err != nil {
			return err
		}
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return err
			}
			path := filepath.Join(dataDir, "video", strconv.FormatInt(videoID, 10), "screenshot", name)
			if err := util.MoveFileToTrash(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := dbpkg.Default().ClearVideoCoverScreenshotNameIfMatch(ctx, videoID, name); err != nil {
				return err
			}
		}
		return nil
	}
}
