package enrichment

import (
	"context"
	"errors"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/jav"
)

// StartUncensoredStudioEnrichment periodically enriches uncensored JAV studios.
func StartUncensoredStudioEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "uncensored jav studios", EnrichUncensoredStudios)
}

// EnrichUncensoredStudios reconciles studio names, preferring English canonical names.
// Only JAVs explicitly marked uncensored are eligible.
func EnrichUncensoredStudios(ctx context.Context) error {
	return enrichUncensoredStudios(ctx, jav.LookupJavByCode)
}

func enrichUncensoredStudios(ctx context.Context, lookup func(context.Context, string, jav.Provider) (*jav.JavInfo, error)) error {
	if common.DB == nil {
		return errors.New("nil db")
	}
	items, err := db.ListJavsNeedingStudioNames(ctx)
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
		if err := enrichJavStudio(ctx, item, []jav.Provider{jav.ProviderAvsox, jav.ProviderJavDBAPI, jav.ProviderJavBus}, lookup); err != nil {
			return err
		}
	}
	_, err = db.UpdateMissingJavSeriesStudios(ctx)
	return err
}
