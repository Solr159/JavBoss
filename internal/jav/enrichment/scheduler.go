package enrichment

import (
	"context"
	"time"

	"javboss/internal/common/logging"
)

type periodicJob func(context.Context) error

func startPeriodicJob(ctx context.Context, interval time.Duration, name string, run periodicJob) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := run(ctx); err != nil {
				logging.Error("%s enrichment failed: %v", name, err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
