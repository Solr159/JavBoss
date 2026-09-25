package service

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
)

type periodicScanFunc func(context.Context) error

// StartJavMetadataScanner periodically promotes studio names to English.
func StartJavMetadataScanner(ctx context.Context, interval time.Duration) {
	startPeriodicScanner(ctx, interval, "jav metadata", ScanJavMetadata)
}

// StartUncensoredJavMetadataScanner periodically fills uncensored metadata through AVSOX.
func StartUncensoredJavMetadataScanner(ctx context.Context, interval time.Duration) {
	startPeriodicScanner(ctx, interval, "uncensored jav metadata", ScanUncensoredJavMetadata)
}

// StartJavSeriesAndIdolMetadataScanner periodically fills missing series and idols through JavDB API.
func StartJavSeriesAndIdolMetadataScanner(ctx context.Context, interval time.Duration) {
	startPeriodicScanner(ctx, interval, "jav series and idol metadata", ScanJavSeriesAndIdolMetadata)
}

func startPeriodicScanner(ctx context.Context, interval time.Duration, name string, scan periodicScanFunc) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := scan(ctx); err != nil {
				logging.Error("%s scan failed: %v", name, err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// ScanJavMetadata promotes studio names using JavDatabase metadata.
func ScanJavMetadata(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}

	return backfillJavEnglishStudioNames(ctx)
}

// backfillJavEnglishStudioNames promotes studio names to English.
func backfillJavEnglishStudioNames(ctx context.Context) error {
	items, err := db.ListJavsNeedingEnglishStudioNameBackfill(ctx)
	if err != nil {
		return err
	}
	shuffleJavMetadataScanItems(items)
	for _, item := range items {
		info, code, ok, err := lookupJavDatabaseMetadata(ctx, item)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		studio := ""
		if info != nil {
			studio = strings.TrimSpace(info.Studio)
		}
		if studio != "" {
			if updated, err := db.PromoteJavStudioEnglishName(ctx, item.ID, studio); err != nil {
				logging.Error("update jav studio failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("jav studio English name updated id=%d code=%s studio=%s", item.ID, code, studio)
			}
		}
	}
	return nil
}

func lookupJavDatabaseMetadata(ctx context.Context, item db.JavMetadataScanItem) (*jav.JavInfo, string, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", false, err
	}
	code := strings.TrimSpace(item.Code)
	if code == "" {
		return nil, "", false, nil
	}

	info, err := jav.LookupJavByCode(code, jav.ProviderJavDatabase)
	if err != nil {
		if !errors.Is(err, jav.ResourceNotFonud) {
			logging.Error("lookup javdatabase metadata failed id=%d code=%s err=%v", item.ID, code, err)
		}
		return nil, code, false, nil
	}
	return info, code, true, nil
}

// ScanJavSeriesAndIdolMetadata fills missing series and idols for all censor states.
func ScanJavSeriesAndIdolMetadata(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}
	items, err := db.ListJavsMissingSeriesOrIdols(ctx)
	if err != nil {
		return err
	}
	logging.Info("found %d javs missing series or idols for javdb-api", len(items))
	shuffleJavMetadataScanItems(items)
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		info, err := jav.LookupJavByCode(code, jav.ProviderJavDBAPI)
		if err != nil {
			if !errors.Is(err, jav.ResourceNotFonud) {
				logging.Error("lookup javdb-api series and idols failed id=%d code=%s err=%v", item.ID, code, err)
			}
			continue
		}
		if info == nil {
			continue
		}
		if series := strings.TrimSpace(info.Series); item.SeriesID == nil && series != "" {
			if updated, err := db.UpdateJavSeriesIfMissing(ctx, item.ID, series); err != nil {
				logging.Error("update javdb-api series failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("jav series updated provider=javdb-api id=%d code=%s series=%s", item.ID, code, series)
			}
		}
		if len(info.Actors) > 0 {
			if updated, err := db.AppendJavIdolsIfMissingForProvider(ctx, item.ID, info.Actors, jav.ProviderJavDBAPI); err != nil {
				logging.Error("update javdb-api idols failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("jav idols updated provider=javdb-api id=%d code=%s count=%d", item.ID, code, len(info.Actors))
			}
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

// ScanUncensoredJavMetadata fills missing uncensored metadata through AVSOX.
func ScanUncensoredJavMetadata(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}
	logging.Info("starting uncensored jav metadata scan")
	return scanMissingUncensoredJavInfoWithAvsox(ctx)
}

func scanMissingUncensoredJavInfoWithAvsox(ctx context.Context) error {
	items, err := db.ListUncensoredJavsMissingAvsoxMetadata(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}

		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}

		info, err := jav.LookupJavByCode(code, jav.ProviderAvsox)
		if err != nil {
			if !errors.Is(err, jav.ResourceNotFonud) {
				logging.Error("lookup avsox uncensored metadata failed id=%d code=%s err=%v", item.ID, code, err)
			}
			continue
		}
		if info == nil {
			continue
		}

		studio := strings.TrimSpace(info.Studio)
		if item.StudioID == nil && studio != "" {
			if updated, err := db.UpdateJavStudioIfMissing(ctx, item.ID, studio); err != nil {
				logging.Error("update uncensored jav studio failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("uncensored jav studio updated provider=%s id=%d code=%s studio=%s", jav.ProviderAvsox.String(), item.ID, code, studio)
			}
		}

		series := strings.TrimSpace(info.Series)
		if item.SeriesID == nil && series != "" {
			if updated, err := db.UpdateJavSeriesIfMissing(ctx, item.ID, series); err != nil {
				logging.Error("update uncensored jav series failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("uncensored jav series updated provider=%s id=%d code=%s series=%s", jav.ProviderAvsox.String(), item.ID, code, series)
			}
		}

		if len(info.Actors) > 0 {
			updated, err := db.AppendJavIdolsIfMissingForProvider(ctx, item.ID, info.Actors, jav.ProviderAvsox)
			if err != nil {
				logging.Error("update uncensored jav idols failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("uncensored jav idols updated provider=%s id=%d code=%s count=%d", jav.ProviderAvsox.String(), item.ID, code, len(info.Actors))
			}
		}
	}
	return nil
}

func shuffleJavMetadataScanItems(items []db.JavMetadataScanItem) {
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
}
