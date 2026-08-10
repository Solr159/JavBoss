package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/western"
)

const westernMetadataPageSize = 100

var westernMetadataScanState struct {
	sync.Mutex
	running bool
}

// StartAutomaticWesternMetadataScheduler periodically fills missing Western
// metadata in the background, similar to an Emby library refresh task.
func StartAutomaticWesternMetadataScheduler(ctx context.Context, pollInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			if err := scanMissingWesternMetadata(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logging.Error("automatic Western metadata scan failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func scanMissingWesternMetadata(ctx context.Context) error {
	token := os.Getenv("JAVBOSS_THEPORNDB_TOKEN")
	if token == "" {
		return nil
	}

	westernMetadataScanState.Lock()
	if westernMetadataScanState.running {
		westernMetadataScanState.Unlock()
		return nil
	}
	westernMetadataScanState.running = true
	westernMetadataScanState.Unlock()
	defer func() {
		westernMetadataScanState.Lock()
		westernMetadataScanState.running = false
		westernMetadataScanState.Unlock()
	}()

	processed := 0
	for offset := 0; ; offset += westernMetadataPageSize {
		videos, err := db.ListVideos(ctx, westernMetadataPageSize, offset, nil, "", "recent", nil, nil, true)
		if err != nil {
			return err
		}
		if len(videos) == 0 {
			break
		}
		for index := range videos {
			if err := ctx.Err(); err != nil {
				return err
			}
			video := &videos[index]
			if video.MediaCategory != models.MediaCategoryWestern && western.IsLikelyJAVFilename(video.Filename) {
				if video.WesternMetadata != nil {
					if err := db.DeleteWesternMetadata(ctx, video.ID); err != nil {
						logging.Error("remove incorrect Western metadata video=%d: %v", video.ID, err)
					}
				}
				if location, err := db.GetPrimaryVideoLocation(ctx, video.ID); err == nil && location != nil {
					videoPath := filepath.Join(location.DirectoryRef.Path, filepath.FromSlash(location.RelativePath))
					if err := western.RemoveNFO(videoPath); err != nil {
						logging.Error("remove incorrect Western NFO video=%d: %v", video.ID, err)
					}
				}
				continue
			}
			if video.MediaCategory == models.MediaCategoryJAV || (video.MediaCategory != models.MediaCategoryWestern && video.Jav != nil) {
				continue
			}
			if video.WesternMetadata != nil && !westernMetadataNeedsRefresh(video.WesternMetadata) {
				continue
			}
			if err := scrapeWesternVideo(ctx, token, video); err != nil {
				if errors.Is(err, western.ErrThePornDBUnauthorized) {
					return err
				}
				logging.Error("automatic Western metadata scrape failed video=%d filename=%s: %v", video.ID, video.Filename, err)
				continue
			}
			processed++
		}
		if len(videos) < westernMetadataPageSize {
			break
		}
	}
	if processed > 0 {
		logging.Info("automatic Western metadata scan complete matched=%d", processed)
	}
	return nil
}

func westernMetadataNeedsRefresh(metadata *models.WesternMetadata) bool {
	if metadata == nil {
		return true
	}
	return metadata.MatchStatus == "matched" && len(metadata.Genres) == 0 && len(metadata.Labels) == 0
}

func scrapeWesternVideo(ctx context.Context, token string, video *models.Video) error {
	location, err := db.GetPrimaryVideoLocation(ctx, video.ID)
	if err != nil {
		return err
	}
	if location == nil {
		return nil
	}
	videoPath := filepath.Join(location.DirectoryRef.Path, filepath.FromSlash(location.RelativePath))
	hash, _ := western.OpenSubtitlesHash(videoPath)
	items, err := western.SearchThePornDBWithOptions(ctx, token, western.SearchOptions{
		Query: video.Filename,
		Hash:  hash,
	})
	if err != nil {
		return err
	}
	metadata := western.Metadata{
		Title:       video.Filename,
		Source:      "theporndb",
		MatchStatus: "unmatched",
		Labels:      []string{"Missing From ThePornDB"},
		Genres:      []string{"Missing From ThePornDB"},
	}
	if len(items) > 0 {
		metadata = items[0]
	}
	if _, err := db.SaveWesternMetadata(ctx, video.ID, metadata); err != nil {
		return err
	}
	return western.WriteNFO(videoPath, metadata)
}
