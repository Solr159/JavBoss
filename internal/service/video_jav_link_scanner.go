package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/jav"
	"pornboss/internal/util"
)

// ScanAll scans video locations that may need a JAV binding.
// It extracts possible JAV codes from each video filename, skips short videos and videos already
// linked through the current preferred provider, first tries to link to an existing jav row, then
// queries the preferred lookup provider, saves any newly found JAV metadata, links the video
// location to that JAV record, and queues the cover image for download.
func ScanAll(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}

	videos, err := db.ListVideosForJavScan(ctx)
	if err != nil {
		return err
	}
	for _, v := range videos {
		if err := ctx.Err(); err != nil {
			return err
		}

		preferredProvider := jav.PreferredProvider()

		if v.JavID != nil && jav.ParseProvider(v.JavProvider) == preferredProvider {
			continue
		}
		if v.DurationSec > 0 && v.DurationSec < 3600 {
			continue
		}

		filename := filepath.Base(filepath.FromSlash(v.Filename))
		possibleCodes := util.ExtractCodeFromName(filename)
		if len(possibleCodes) == 0 {
			continue
		}

		linked := false
		for _, code := range possibleCodes {
			if existJav, err := db.GetJavByCode(ctx, code); err == nil && existJav != nil {
				if jav.ParseProvider(existJav.Provider) != preferredProvider {
					continue
				}
				if err := db.SetVideoLocationJavID(ctx, v.LocationID, existJav.ID, v.UpdatedAt); err != nil {
					logging.Error("set video location jav failed location=%d code=%s err=%v", v.LocationID, code, err)
				} else {
					enqueueCover(existJav.Code)
				}
				linked = true
				break
			} else if err != nil {
				logging.Error("jav lookup existing failed location=%d code=%s err=%v", v.LocationID, code, err)
			}
		}
		if linked {
			continue
		}

		for _, code := range possibleCodes {
			info, err := jav.LookupJavByCode(code, preferredProvider)
			if err != nil {
				if errors.Is(err, jav.ResourceNotFonud) {
					continue
				}
				logging.Error("jav lookup failed location=%s code=%s err=%v", filename, code, err)
				continue
			}
			if info == nil {
				continue
			}

			_, err = db.SaveJavInfoAndLinkLocation(ctx, info, v.LocationID, v.UpdatedAt)
			if err != nil {
				logging.Error("link video location->jav failed location=%s code=%s err=%v", filename, info.Code, err)
			} else {
				logging.Info("link video location->jav success location=%s code=%s", filename, info.Code)
				enqueueCover(info.Code)
			}
			linked = true
			break
		}
	}
	return nil
}

// StartJavScanner periodically scans videos and tries to bind them to JAV records.
// It runs ScanAll immediately and then on every interval until ctx is done; after each binding
// pass it also scans known JAV records for missing cover images and enqueues those covers.
func StartJavScanner(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := ScanAll(ctx); err != nil {
				logging.Error("periodic jav scan failed: %v", err)
			}
			if err := enqueueMissingCovers(ctx); err != nil {
				logging.Error("periodic jav cover enqueue failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func enqueueCover(code string) {
	mgr := common.CoverManager
	if mgr == nil {
		return
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	mgr.Enqueue(code)
}

func enqueueMissingCovers(ctx context.Context) error {
	mgr := common.CoverManager
	if common.DB == nil || mgr == nil {
		return nil
	}
	codes, err := db.ListJavCodes(ctx)
	if err != nil {
		return err
	}
	for _, c := range codes {
		code := strings.TrimSpace(c)
		if code == "" {
			continue
		}
		if mgr.Exists(code) {
			continue
		}
		mgr.Enqueue(code)
	}
	return nil
}
