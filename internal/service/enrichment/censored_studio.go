package enrichment

import (
	"context"
	"errors"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/jav"
)

// StartCensoredStudioEnrichment periodically enriches censored JAV studios.
func StartCensoredStudioEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "censored jav studios", EnrichCensoredStudios)
}

// EnrichCensoredStudios reconciles studio names, preferring English canonical names.
// Unknown censor states are treated as censored.
func EnrichCensoredStudios(ctx context.Context) error {
	return enrichCensoredStudios(ctx, jav.LookupJavByCode)
}

func enrichCensoredStudios(ctx context.Context, lookup func(context.Context, string, jav.Provider) (*jav.JavInfo, error)) error {
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
		if (item.IsUncensored != nil && *item.IsUncensored) != false {
			continue
		}
		if err := enrichJavStudio(ctx, item, []jav.Provider{jav.ProviderJavDatabase, jav.ProviderJavDBAPI}, lookup); err != nil {
			return err
		}
	}
	_, err = db.UpdateMissingJavSeriesStudios(ctx)
	return err
}
