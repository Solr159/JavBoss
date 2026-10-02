package enrichment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
)

// StartCensoredSeriesEnrichment periodically enriches censored JAV series.
func StartCensoredSeriesEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "censored jav series", EnrichCensoredSeries)
}

// EnrichCensoredSeries fills missing series through JavDB API, then JavMenu.
// The third source probes JavDatabase for a series before extracting it from Avmoo.
// Unknown censor states are treated as censored.
func EnrichCensoredSeries(ctx context.Context) error {
	return enrichCensoredSeries(ctx, jav.LookupJavByCode)
}

func enrichCensoredSeries(ctx context.Context, lookup func(context.Context, string, jav.Provider) (*jav.JavInfo, error)) error {
	if common.DB == nil {
		return errors.New("nil db")
	}
	items, err := db.ListJavsMissingSeries(ctx)
	if err != nil {
		return err
	}
	shuffleCandidates(items)
	providers := []struct {
		name   string
		lookup func(context.Context, string) (*jav.JavInfo, error)
	}{
		{"javdb-api", func(ctx context.Context, code string) (*jav.JavInfo, error) {
			return lookup(ctx, code, jav.ProviderJavDBAPI)
		}},
		{"javmenu", func(ctx context.Context, code string) (*jav.JavInfo, error) {
			return lookup(ctx, code, jav.ProviderJavMenu)
		}},
		{"javdatabase -> avmoo", func(ctx context.Context, code string) (*jav.JavInfo, error) {
			return lookupAvmooSeriesAfterJavDatabase(ctx, code, lookup)
		}},
	}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if (item.IsUncensored != nil && *item.IsUncensored) != false {
			continue
		}
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		for _, provider := range providers {
			info, err := provider.lookup(ctx, code)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil {
				if !errors.Is(err, jav.ErrNotFound) {
					logging.Error("lookup jav series failed provider=%s id=%d code=%s err=%v", provider.name, item.ID, code, err)
				}
				continue
			}
			if info == nil {
				continue
			}
			series := strings.TrimSpace(info.Series)
			if series == "" {
				continue
			}
			if updated, err := db.UpdateJavSeriesIfMissing(ctx, item.ID, series); err != nil {
				logging.Error("update jav series failed provider=%s id=%d code=%s err=%v", provider.name, item.ID, code, err)
				continue
			} else if updated {
				logging.Info("jav series updated provider=%s id=%d code=%s", provider.name, item.ID, code)
			}
			// A valid result either filled the field or an existing value was preserved.
			break
		}
	}
	return nil
}

// JavDatabase only confirms that a series exists. Persist the series from Avmoo,
// never the English probe result.
func lookupAvmooSeriesAfterJavDatabase(ctx context.Context, code string, lookup func(context.Context, string, jav.Provider) (*jav.JavInfo, error)) (*jav.JavInfo, error) {
	info, err := lookup(ctx, code, jav.ProviderJavDatabase)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, fmt.Errorf("probe javdatabase series: %w", err)
	}
	if info == nil || strings.TrimSpace(info.Series) == "" {
		return nil, nil
	}
	info, err = lookup(ctx, code, jav.ProviderAvmoo)
	if err != nil {
		return nil, fmt.Errorf("lookup avmoo series: %w", err)
	}
	return info, nil
}
