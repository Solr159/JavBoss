package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/jav"
)

// StartJavMetadataScanner periodically fills missing JAV studio and series metadata.
func StartJavMetadataScanner(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := ScanJavMetadata(ctx); err != nil {
				logging.Error("jav metadata scan failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// ScanJavMetadata scans JAV rows with missing studio or series data and queries metadata providers for it.
func ScanJavMetadata(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}

	items, err := db.ListJavsMissingStudioOrSeries(ctx)
	if err != nil {
		return err
	}
	if len(items) > 0 {
		logging.Info("found %d jav rows missing studio or series info", len(items))

		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return err
			}

			code := strings.TrimSpace(item.Code)
			if code == "" {
				continue
			}

			info, err := jav.LookupJavByCode(code, jav.ProviderJavDatabase)
			if err != nil {
				if !errors.Is(err, jav.ResourceNotFonud) {
					logging.Error("lookup javdatabase metadata failed id=%d code=%s err=%v", item.ID, code, err)
				}
				continue
			}

			studio := ""
			seriesEn := ""
			if info != nil {
				studio = strings.TrimSpace(info.Studio)
				seriesEn = strings.TrimSpace(info.Series)
			}
			if studio != "" {
				if err := db.UpdateJavStudio(ctx, item.ID, studio); err != nil {
					logging.Error("update jav studio failed id=%d code=%s err=%v", item.ID, code, err)
				} else {
					logging.Info("jav studio updated id=%d code=%s studio=%s", item.ID, code, studio)
				}
			}

			updatedEnglishSeries := false
			if seriesEn != "" {
				if err := db.UpdateJavSeries(ctx, item.ID, seriesEn, true); err != nil {
					logging.Error("update jav english series failed id=%d code=%s err=%v", item.ID, code, err)
				} else {
					updatedEnglishSeries = true
					logging.Info("jav english series updated id=%d code=%s series=%s", item.ID, code, seriesEn)
				}
			}
			if !updatedEnglishSeries {
				continue
			}

			avmooInfo, err := jav.LookupJavByCode(code, jav.ProviderAvmoo)
			if err != nil {
				if !errors.Is(err, jav.ResourceNotFonud) {
					logging.Error("lookup avmoo metadata failed id=%d code=%s err=%v", item.ID, code, err)
				}
				continue
			}
			series := ""
			if avmooInfo != nil {
				series = strings.TrimSpace(avmooInfo.Series)
			}
			if series == "" {
				continue
			}
			if err := db.UpdateJavSeries(ctx, item.ID, series, false); err != nil {
				logging.Error("update jav series failed id=%d code=%s err=%v", item.ID, code, err)
				continue
			}
			logging.Info("jav series updated id=%d code=%s series=%s", item.ID, code, series)
		}
	}

	updated, err := db.UpdateMissingJavSeriesStudios(ctx)
	if err != nil {
		return err
	}
	if updated > 0 {
		logging.Info("updated %d jav series studio ids", updated)
	}
	return nil
}
