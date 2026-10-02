package enrichment

import (
	"context"
	"errors"
	"strings"
	"sync"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
)

func enrichJavStudio(ctx context.Context, item db.JavEnrichmentItem, providers []jav.Provider, lookup func(context.Context, string, jav.Provider) (*jav.JavInfo, error)) error {
	code := strings.TrimSpace(item.Code)
	if code == "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Each provider writes its own slot so name priority is independent of
	// response order. Only one movie's providers are in flight at a time.
	names := make([]string, len(providers))
	var pending sync.WaitGroup
	for i, provider := range providers {
		pending.Add(1)
		go func() {
			defer pending.Done()
			info, err := lookup(ctx, code, provider)
			if err != nil {
				if ctx.Err() == nil && !errors.Is(err, jav.ErrNotFound) {
					logging.Error("lookup jav studio failed provider=%s id=%d code=%s err=%v", provider, item.ID, code, err)
				}
				return
			}
			if info != nil {
				names[i] = strings.TrimSpace(info.Studio)
			}
		}()
	}
	pending.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if updated, err := db.ReconcileJavStudioNames(ctx, item.ID, item.StudioID, names); err != nil {
		return err
	} else if updated {
		logging.Info("jav studio names reconciled id=%d code=%s names=%q", item.ID, code, names)
	}
	return nil
}
