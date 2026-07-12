package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
)

const javUncensoredBackfillDoneConfigKey = "jav_uncensored_backfill_done"

type javMetadataScanFunc func(context.Context) error

// StartJavMetadataScanner periodically fills missing JAV metadata using the fast providers.
func StartJavMetadataScanner(ctx context.Context, interval time.Duration) {
	startJavMetadataScanner(ctx, interval, "jav metadata", ScanJavMetadata)
}

// StartSlowJavMetadataScanner periodically fills metadata from slow JAV providers.
func StartSlowJavMetadataScanner(ctx context.Context, interval time.Duration) {
	startJavMetadataScanner(ctx, interval, "slow jav metadata", ScanSlowJavMetadata)
}

func startJavMetadataScanner(ctx context.Context, interval time.Duration, name string, scan javMetadataScanFunc) {
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

// ScanJavMetadata scans JAV rows with missing title, English title, studio, or series data and queries fast metadata providers for it.
func ScanJavMetadata(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}

	if err := scanMissingJavZhInfo(ctx, javFastZhMetadataProviders()); err != nil {
		return err
	}
	if err := scanMissingJavUncensoredBackfillOnce(ctx); err != nil {
		return err
	}
	if jav.CurrentMetadataLanguageIsEnglish() {
		if err := scanMissingJavEnglishMetadata(ctx); err != nil {
			return err
		}
	} else if err := scanMissingJavStudioAndEnglishSeries(ctx); err != nil {
		return err
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

func scanMissingJavStudioAndEnglishSeries(ctx context.Context) error {
	items, err := db.ListJavsMissingStudioOrEnglishSeries(ctx)
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
		seriesEn := ""
		if info != nil {
			studio = strings.TrimSpace(info.Studio)
			seriesEn = strings.TrimSpace(info.Series)
		}
		if item.StudioID == nil && studio != "" {
			if updated, err := db.UpdateJavStudioIfMissing(ctx, item.ID, studio); err != nil {
				logging.Error("update jav studio failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("jav studio updated id=%d code=%s studio=%s", item.ID, code, studio)
			}
		}
		if item.SeriesEnID == nil && seriesEn != "" {
			if updated, err := db.UpdateJavSeriesIfMissing(ctx, item.ID, seriesEn, true); err != nil {
				logging.Error("update jav english series failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("jav english series updated id=%d code=%s series=%s", item.ID, code, seriesEn)
			}
		}
	}
	return nil
}

func updateMissingJavStudio(ctx context.Context, item db.JavMetadataScanItem, code, studio string) {
	if item.StudioID != nil || studio == "" {
		return
	}
	if updated, err := db.UpdateJavStudioIfMissing(ctx, item.ID, studio); err != nil {
		logging.Error("update jav studio failed id=%d code=%s err=%v", item.ID, code, err)
	} else if updated {
		logging.Info("jav studio updated id=%d code=%s studio=%s", item.ID, code, studio)
	}
}

func scanMissingJavEnglishMetadata(ctx context.Context) error {
	items, err := db.ListJavsMissingEnglishMetadata(ctx)
	if err != nil {
		return err
	}
	shuffleJavMetadataScanItems(items)
	for _, item := range items {
		info, code, ok, err := lookupJavDatabaseMetadata(ctx, item)
		if err != nil {
			return err
		}
		if !ok || info == nil {
			continue
		}

		titleEn := strings.TrimSpace(info.Title)
		studio := strings.TrimSpace(info.Studio)
		seriesEn := strings.TrimSpace(info.Series)
		updatedEnglishMetadata := false
		if strings.TrimSpace(item.TitleEn) == "" && titleEn != "" {
			if _, err := db.SaveJavInfo(ctx, info); err != nil {
				logging.Error("update jav english metadata failed id=%d code=%s err=%v", item.ID, code, err)
			} else {
				updatedEnglishMetadata = true
				logging.Info("jav english metadata updated id=%d code=%s title=%s", item.ID, code, titleEn)
			}
		}
		if item.SeriesEnID == nil && seriesEn != "" && !updatedEnglishMetadata {
			if updated, err := db.UpdateJavSeriesIfMissing(ctx, item.ID, seriesEn, true); err != nil {
				logging.Error("update jav english series failed id=%d code=%s err=%v", item.ID, code, err)
			} else if updated {
				logging.Info("jav english series updated id=%d code=%s series=%s", item.ID, code, seriesEn)
			}
		}
		updateMissingJavStudio(ctx, item, code, studio)
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

// ScanSlowJavMetadata scans JAV rows using providers that are known to be slow or heavily rate limited.
func ScanSlowJavMetadata(ctx context.Context) error {
	logging.Info("starting slow jav metadata scan")

	if err := scanMissingUncensoredJavInfoWithAvsox(ctx); err != nil {
		return err
	}
	if err := scanMissingJavLocalSeriesWithAvmoo(ctx); err != nil {
		return err
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

func javFastZhMetadataProviders() []jav.Provider {
	return []jav.Provider{jav.ProviderJavBus}
}

func scanMissingJavUncensoredBackfillOnce(ctx context.Context) error {
	done, err := javUncensoredBackfillDone(ctx)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	if err := scanMissingJavUncensored(ctx); err != nil {
		return err
	}
	if err := db.UpsertConfig(ctx, map[string]string{javUncensoredBackfillDoneConfigKey: "1"}); err != nil {
		return fmt.Errorf("mark jav uncensored backfill done: %w", err)
	}
	logging.Info("jav uncensored backfill marked done")
	return nil
}

func javUncensoredBackfillDone(ctx context.Context) (bool, error) {
	entries, err := db.ListConfig(ctx)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(entries[javUncensoredBackfillDoneConfigKey]) == "1", nil
}

// jav表uncensored字段是新增的，存量数据中使用javbus获取的jav的uncensored状态未知，这个函数专门用javbus重新获取一遍来补齐这个信息。
func scanMissingJavUncensored(ctx context.Context) error {
	items, err := db.ListJavsMissingUncensored(ctx)
	if err != nil {
		return err
	}
	shuffleJavMetadataScanItems(items)
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}

		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}

		for _, provider := range []jav.Provider{jav.ProviderJavBus} {
			info, err := jav.LookupJavByCode(code, provider)
			if err != nil {
				if !errors.Is(err, jav.ResourceNotFonud) {
					logging.Error("lookup %s uncensored state failed id=%d code=%s err=%v", provider.String(), item.ID, code, err)
				}
				continue
			}
			if info == nil || info.IsUncensored == nil {
				continue
			}
			if err := db.UpdateJavIsUncensoredIfUnknown(ctx, item.ID, *info.IsUncensored); err != nil {
				logging.Error("update jav is_uncensored failed provider=%s id=%d code=%s err=%v", provider.String(), item.ID, code, err)
				continue
			}
			logging.Info("jav is_uncensored updated provider=%s id=%d code=%s is_uncensored=%t", provider.String(), item.ID, code, *info.IsUncensored)
			break
		}
	}
	return nil
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
			if updated, err := db.UpdateJavSeriesIfMissing(ctx, item.ID, series, false); err != nil {
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

func scanMissingJavLocalSeriesWithAvmoo(ctx context.Context) error {
	logging.Info("starting scan missing jav local series with avmoo")
	items, err := db.ListJavsMissingLocalSeriesWithEnglishSeries(ctx)
	if err != nil {
		return err
	}
	logging.Info("found %d javs missing local series with english series", len(items))
	shuffleJavMetadataScanItems(items)
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}

		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}

		info, err := jav.LookupJavByCode(code, jav.ProviderAvmoo)
		if err != nil {
			if !errors.Is(err, jav.ResourceNotFonud) {
				logging.Error("lookup avmoo metadata failed id=%d code=%s err=%v", item.ID, code, err)
			}
			continue
		}

		series := ""
		if info != nil {
			series = strings.TrimSpace(info.Series)
		}
		if series == "" {
			continue
		}
		if updated, err := db.UpdateJavSeriesIfMissing(ctx, item.ID, series, false); err != nil {
			logging.Error("update jav local series failed id=%d code=%s err=%v", item.ID, code, err)
			continue
		} else if updated {
			logging.Info("jav local series updated id=%d code=%s series=%s", item.ID, code, series)
		}
	}
	return nil
}

func shuffleJavMetadataScanItems(items []db.JavMetadataScanItem) {
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
}

// 某些信息可能是通过英文数据源javdatabase获取的，缺少中日文元数据信息，这个函数专门用来补齐。
func scanMissingJavZhInfo(ctx context.Context, providers []jav.Provider) error {
	items, err := db.ListJavsMissingTitle(ctx)
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

		for _, provider := range providers {
			info, err := jav.LookupJavByCode(code, provider)
			if err != nil {
				if !errors.Is(err, jav.ResourceNotFonud) {
					logging.Error("lookup %s metadata failed id=%d code=%s err=%v", provider.String(), item.ID, code, err)
				}
				continue
			}
			if info == nil || strings.TrimSpace(info.Title) == "" {
				continue
			}
			if _, err := db.SaveJavInfo(ctx, info); err != nil {
				logging.Error("update jav title metadata failed provider=%s id=%d code=%s err=%v", provider.String(), item.ID, code, err)
				continue
			}
			logging.Info("jav title metadata updated provider=%s id=%d code=%s title=%s", provider.String(), item.ID, code, strings.TrimSpace(info.Title))
			break
		}
	}
	return nil
}
