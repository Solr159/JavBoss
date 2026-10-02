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

// StartUncensoredIdolEnrichment periodically enriches uncensored JAV idols.
func StartUncensoredIdolEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "uncensored jav idols", EnrichUncensoredIdols)
}

// EnrichUncensoredIdols fills missing idols through JavDB API, then AVSOX.
// Only JAVs explicitly marked uncensored are eligible.
func EnrichUncensoredIdols(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}
	items, err := db.ListJavsMissingIdols(ctx)
	if err != nil {
		return err
	}
	shuffleCandidates(items)
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if (item.IsUncensored != nil && *item.IsUncensored) != true {
			continue
		}
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		for _, provider := range []jav.Provider{jav.ProviderJavDBAPI, jav.ProviderAvsox} {
			info, err := jav.LookupJavByCode(ctx, code, provider)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil {
				if !errors.Is(err, jav.ErrNotFound) {
					logging.Error("lookup jav idols failed provider=%s id=%d code=%s err=%v", provider, item.ID, code, err)
				}
				continue
			}
			if info == nil {
				continue
			}
			var names []string
			for _, name := range info.Actors {
				if name = strings.TrimSpace(name); name != "" {
					names = append(names, name)
				}
			}
			if len(names) == 0 {
				continue
			}
			if updated, err := db.AppendJavIdolsIfMissingForProvider(ctx, item.ID, names, provider); err != nil {
				logging.Error("update jav idols failed provider=%s id=%d code=%s err=%v", provider, item.ID, code, err)
				continue
			} else if updated {
				logging.Info("jav idols updated provider=%s id=%d code=%s", provider, item.ID, code)
			}
			// A valid result either filled the field or an existing value was preserved.
			break
		}
	}
	return nil
}
