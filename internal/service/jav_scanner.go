package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
)

// StartJavMetadataScanner periodically fills missing JAV metadata.
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

// ScanJavMetadata scans JAV rows with missing title, English title, studio, or series data and queries metadata providers for it.
// TODO: AVMOO和AVSOX爬虫限流较慢，会拖慢整个扫描过程，后续考虑单独异步处理这两个爬虫相关的信息补齐。
func ScanJavMetadata(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}

	if err := scanMissingJavZhInfo(ctx); err != nil {
		return err
	}
	if err := scanMissingJavUncensored(ctx); err != nil {
		return err
	}
	if err := scanMissingUncensoredJavInfoWithAvsox(ctx); err != nil {
		return err
	}

	items, err := db.ListJavsMissingMetadata(ctx)
	if err != nil {
		return err
	}
	if len(items) > 0 {
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

			titleEn := ""
			studio := ""
			seriesEn := ""
			if info != nil {
				titleEn = strings.TrimSpace(info.Title)
				studio = strings.TrimSpace(info.Studio)
				seriesEn = strings.TrimSpace(info.Series)
			}
			updatedEnglishMetadata := false
			if strings.TrimSpace(item.TitleEn) == "" && titleEn != "" {
				if _, err := db.SaveJavInfo(ctx, info); err != nil {
					logging.Error("update jav english metadata failed id=%d code=%s err=%v", item.ID, code, err)
				} else {
					updatedEnglishMetadata = true
					logging.Info("jav english metadata updated id=%d code=%s title=%s", item.ID, code, titleEn)
				}
			}
			if item.StudioID == nil && studio != "" && !updatedEnglishMetadata {
				if err := db.UpdateJavStudio(ctx, item.ID, studio); err != nil {
					logging.Error("update jav studio failed id=%d code=%s err=%v", item.ID, code, err)
				} else {
					logging.Info("jav studio updated id=%d code=%s studio=%s", item.ID, code, studio)
				}
			}

			updatedEnglishSeries := false
			if item.SeriesEnID == nil && seriesEn != "" {
				if updatedEnglishMetadata {
					updatedEnglishSeries = true
				} else if err := db.UpdateJavSeries(ctx, item.ID, seriesEn, true); err != nil {
					logging.Error("update jav english series failed id=%d code=%s err=%v", item.ID, code, err)
				} else {
					updatedEnglishSeries = true
					logging.Info("jav english series updated id=%d code=%s series=%s", item.ID, code, seriesEn)
				}
			}
			if item.SeriesID != nil || (item.SeriesEnID == nil && !updatedEnglishSeries) {
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

// jav表uncensored字段是新增的，存量数据中使用javbus获取的jav的uncensored状态未知，这个函数专门用javbus重新获取一遍来补齐这个信息。
func scanMissingJavUncensored(ctx context.Context) error {
	items, err := db.ListJavsMissingUncensored(ctx)
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
			if err := db.UpdateJavStudio(ctx, item.ID, studio); err != nil {
				logging.Error("update uncensored jav studio failed id=%d code=%s err=%v", item.ID, code, err)
			} else {
				logging.Info("uncensored jav studio updated provider=%s id=%d code=%s studio=%s", jav.ProviderAvsox.String(), item.ID, code, studio)
			}
		}

		series := strings.TrimSpace(info.Series)
		if item.SeriesID == nil && series != "" {
			if err := db.UpdateJavSeries(ctx, item.ID, series, false); err != nil {
				logging.Error("update uncensored jav series failed id=%d code=%s err=%v", item.ID, code, err)
			} else {
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

// 某些信息可能是通过英文数据源javdatabase获取的，缺少中日文元数据信息，这个函数专门用来补齐。
func scanMissingJavZhInfo(ctx context.Context) error {
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

		for _, provider := range []jav.Provider{jav.ProviderJavBus, jav.ProviderAvmoo} {
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
