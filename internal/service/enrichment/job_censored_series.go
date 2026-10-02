package enrichment

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

// StartCensoredSeriesEnrichment periodically enriches censored JAV series.
func StartCensoredSeriesEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "censored jav series", EnrichCensoredSeries)
}

// EnrichCensoredSeries fills missing series through JavDB API, then JavMenu.
// Unknown censor states are treated as censored.
func EnrichCensoredSeries(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}
	items, err := db.ListJavsMissingSeries(ctx)
	if err != nil {
		return err
	}
	shuffleCandidates(items)
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
		for _, provider := range []jav.Provider{jav.ProviderJavDBAPI, jav.ProviderJavMenu} {
			info, err := jav.LookupJavByCode(ctx, code, provider)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil {
				if !errors.Is(err, jav.ErrNotFound) {
					logging.Error("lookup jav series failed provider=%s id=%d code=%s err=%v", provider, item.ID, code, err)
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
				logging.Error("update jav series failed provider=%s id=%d code=%s err=%v", provider, item.ID, code, err)
				continue
			} else if updated {
				logging.Info("jav series updated provider=%s id=%d code=%s", provider, item.ID, code)
			}
			// A valid result either filled the field or an existing value was preserved.
			break
		}
	}
	return nil
}
