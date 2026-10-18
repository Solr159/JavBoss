package enrichment

import (
	"context"
	"time"

	"javboss/internal/db"
	"javboss/internal/jav/scrape"
)

// StartUncensoredStudioEnrichment periodically enriches uncensored JAV studios.
func StartUncensoredStudioEnrichment(ctx context.Context, interval time.Duration) {
	startPeriodicJob(ctx, interval, "uncensored jav studios", EnrichUncensoredStudios)
}

// EnrichUncensoredStudios reconciles studio names, preferring English canonical names.
// Only JAVs explicitly marked uncensored are eligible.
func EnrichUncensoredStudios(ctx context.Context) error {
	return enrichUncensoredStudios(ctx, scrape.LookupJavByCode)
}

func enrichUncensoredStudios(ctx context.Context, lookup func(context.Context, string, scrape.Provider) (*scrape.JavInfo, error)) error {
	items, err := db.Default().ListJavsNeedingStudioNames(ctx)
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
		if err := enrichJavStudio(ctx, item, []scrape.Provider{scrape.ProviderAvsox, scrape.ProviderJavDBAPI, scrape.ProviderJavBus}, lookup); err != nil {
			return err
		}
	}
	_, err = db.Default().UpdateMissingJavSeriesStudios(ctx)
	return err
}
