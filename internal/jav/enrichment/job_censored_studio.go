package enrichment

import (
	"context"
	"time"

	"javboss/internal/db"
	"javboss/internal/jav/scrape"
)

// StartCensoredStudioEnrichment periodically enriches censored JAV studios.
func StartCensoredStudioEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "censored jav studios", EnrichCensoredStudios)
}

// EnrichCensoredStudios reconciles studio names, preferring English canonical names.
// Unknown censor states are treated as censored.
func EnrichCensoredStudios(ctx context.Context) error {
	return enrichCensoredStudios(ctx, scrape.LookupJavByCode)
}

func enrichCensoredStudios(ctx context.Context, lookup func(context.Context, string, scrape.Provider) (*scrape.JavInfo, error)) error {
	items, err := db.Default().ListJavsNeedingStudioNames(ctx)
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
		if err := enrichJavStudio(ctx, item, []scrape.Provider{scrape.ProviderJavDatabase, scrape.ProviderJavDBAPI}, lookup); err != nil {
			return err
		}
	}
	_, err = db.Default().UpdateMissingJavSeriesStudios(ctx)
	return err
}
